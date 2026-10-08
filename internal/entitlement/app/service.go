package app

import (
        "context"
        "crypto/sha256"
        "encoding/hex"
        "fmt"
        "regexp"
        "strconv"
        "strings"
        "time"

        identityAPI "github.com/artemiosu/staffy/internal/identity/api"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
        "github.com/artemiosu/staffy/internal/entitlement/api"
        entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
        "github.com/artemiosu/staffy/internal/entitlement/port"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// IdentityDirectory is the identity surface the module reads: membership
// check plus scope authorization. The entitlement module never writes
// identity tables.
type IdentityDirectory interface {
        identityAPI.MembershipService
        Workspace(ctx context.Context, principal identityDomain.Principal, tenantID id.UUID) (identityDomain.Workspace, error)
}

// Service implements the entitlement application logic. Managing the
// commercial perimeter requires the commercial scope; the full view
// requires commercial or audit; any member sees the summary level; usage
// ingestion requires commercial or operations. Authorization failures
// surface as ErrNotFound so tenant existence is never disclosed.
type Service struct {
        repository port.Repository
        identity   IdentityDirectory
        clock      api.Clock
        ids        api.IDGenerator
}

// New wires the entitlement service over its repository and the identity
// directory.
func New(repository port.Repository, identity IdentityDirectory, clock api.Clock, ids api.IDGenerator) *Service {
        return &Service{repository: repository, identity: identity, clock: clock, ids: ids}
}

func (s *Service) tenancy(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        if !actor.ID.Valid() || !tenantID.Valid() {
                return tenancy.Context{}, entitlementDomain.ErrNotFound
        }
        return tenancy.New(actor.ID, tenantID)
}

// viewLevel authorizes read access: commercial or audit see the full
// detail; any other member sees the summary level only; non-members get an
// honest ErrNotFound.
func (s *Service) viewLevel(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, string, error) {
        scope, err := s.tenancy(ctx, actor, tenantID)
        if err != nil {
                return tenancy.Context{}, "", err
        }
        if _, err := s.identity.Workspace(ctx, actor, tenantID); err != nil {
                return tenancy.Context{}, "", entitlementDomain.ErrNotFound
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Commercial); err == nil {
                return scope, api.LevelFull, nil
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Audit); err == nil {
                return scope, api.LevelFull, nil
        }
        return scope, api.LevelSummary, nil
}

// manageScope authorizes commercial management (terms, states, budgets,
// sources, manual exceptions).
func (s *Service) manageScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        scope, err := s.tenancy(ctx, actor, tenantID)
        if err != nil {
                return tenancy.Context{}, err
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Commercial); err != nil {
                return tenancy.Context{}, entitlementDomain.ErrNotFound
        }
        return scope, nil
}

// ingestScope authorizes usage event ingestion: commercial or operations.
func (s *Service) ingestScope(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (tenancy.Context, error) {
        scope, err := s.tenancy(ctx, actor, tenantID)
        if err != nil {
                return tenancy.Context{}, err
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Commercial); err == nil {
                return scope, nil
        }
        if err := s.identity.Authorize(ctx, actor, tenantID, identityDomain.Operations); err == nil {
                return scope, nil
        }
        return tenancy.Context{}, entitlementDomain.ErrNotFound
}

// loadFacts reads the whole append-only commercial history of the tenant.
func (s *Service) loadFacts(ctx context.Context, scope tenancy.Context) (terms []entitlementDomain.TermsVersion, stateEvents []entitlementDomain.StateEvent, budgets []entitlementDomain.BudgetVersion, sources []entitlementDomain.UsageSource, events []entitlementDomain.UsageEvent, err error) {
        if terms, err = s.repository.ListTerms(ctx, scope); err != nil {
                return
        }
        if stateEvents, err = s.repository.ListStateEvents(ctx, scope); err != nil {
                return
        }
        if budgets, err = s.repository.ListBudgets(ctx, scope); err != nil {
                return
        }
        if sources, err = s.repository.ListSources(ctx, scope); err != nil {
                return
        }
        events, err = s.repository.ListUsageEvents(ctx, scope)
        return
}

func latestTerms(terms []entitlementDomain.TermsVersion) *entitlementDomain.TermsVersion {
        var current *entitlementDomain.TermsVersion
        for i := range terms {
                if current == nil || terms[i].Version > current.Version {
                        current = &terms[i]
                }
        }
        return current
}

func latestBudget(budgets []entitlementDomain.BudgetVersion) *entitlementDomain.BudgetVersion {
        var current *entitlementDomain.BudgetVersion
        for i := range budgets {
                if current == nil || budgets[i].Version > current.Version {
                        current = &budgets[i]
                }
        }
        return current
}

// budgetQuality is the qualitative budget relation of the summary level:
// no sums, no currency, only the honest state.
func budgetQuality(meter entitlementDomain.Meter) string {
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

// EntitlementView composes the derived commercial projection. The draft
// state shows the composition skeleton with honest open questions instead
// of zero prices; the admission state names what continues and who owns
// the next step.
func (s *Service) EntitlementView(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (api.EntitlementView, error) {
        scope, level, err := s.viewLevel(ctx, actor, tenantID)
        if err != nil {
                return api.EntitlementView{}, err
        }
        terms, stateEvents, budgets, sources, events, err := s.loadFacts(ctx, scope)
        if err != nil {
                return api.EntitlementView{}, err
        }
        state := entitlementDomain.DeriveEntitlementState(stateEvents)
        now := s.clock.Now().UTC()
        meter := entitlementDomain.ComputeMeter(events, entitlementDomain.CurrentSources(sources), latestBudget(budgets), now)
        view := api.EntitlementView{
                Level:       level,
                State:       state,
                Composition: entitlementDomain.CategorySpecs(),
                StateEvents: stateEvents,
                Sources:     sources,
                Admission:   entitlementDomain.DeriveAdmission(state, meter.ConfirmedTotal, latestBudget(budgets)),
                BudgetQuality: budgetQuality(meter),
        }
        if current := latestTerms(terms); current != nil {
                view.Terms = current
                view.OpenQuestions = current.OpenQuestions()
                view.History = terms
        } else {
                view.OpenQuestions = []string{
                        "коммерческое предложение не составлено: опишите версию условий",
                        "цена запуска не определена (не ноль, а открытый вопрос)",
                        "цена подписки ИИ-сотрудника не определена (не ноль, а открытый вопрос)",
                }
        }
        if level == api.LevelFull {
                if current := latestBudget(budgets); current != nil {
                        view.Budget = current
                }
        } else {
                // The summary level carries only the entitlement state and the
                // qualitative budget status: no terms, no tariff history, no
                // lifecycle events with commercial grounds, no source
                // registrations (R1: they used to leak through both the JSON
                // API and the page while the page claimed the opposite).
                view.Terms = nil
                view.OpenQuestions = nil
                view.History = nil
                view.StateEvents = nil
                view.Sources = nil
        }
        return view, nil
}

// MeterView composes the derived Budget Meter with honest data statuses.
// The summary level carries no sums, terms or breakdowns.
func (s *Service) MeterView(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (api.MeterView, error) {
        scope, level, err := s.viewLevel(ctx, actor, tenantID)
        if err != nil {
                return api.MeterView{}, err
        }
        _, _, budgets, sources, events, err := s.loadFacts(ctx, scope)
        if err != nil {
                return api.MeterView{}, err
        }
        now := s.clock.Now().UTC()
        meter := entitlementDomain.ComputeMeter(events, entitlementDomain.CurrentSources(sources), latestBudget(budgets), now)
        view := api.MeterView{
                Level:             level,
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
                BudgetQuality:     budgetQuality(meter),
        }
        if level == api.LevelSummary {
                view.Categories = nil
                view.MissingCategories = nil
                view.ConfirmedTotal = nil
                view.EstimateTotal = nil
                view.Budget = nil
                view.Breakdowns = nil
                view.PeriodStart = nil
                view.PeriodEnd = nil
                view.LastIngestedAt = nil
        }
        return view, nil
}

// AdmissionView exposes the derived admission state for future consumers
// (readiness 1-14, Epic 2): a pure function of the entitlement state, the
// confirmed spend and the declared policy. Enforcement is not wired here.
func (s *Service) AdmissionView(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (entitlementDomain.AdmissionState, error) {
        scope, _, err := s.viewLevel(ctx, actor, tenantID)
        if err != nil {
                return entitlementDomain.AdmissionState{}, err
        }
        _, stateEvents, budgets, sources, events, err := s.loadFacts(ctx, scope)
        if err != nil {
                return entitlementDomain.AdmissionState{}, err
        }
        state := entitlementDomain.DeriveEntitlementState(stateEvents)
        meter := entitlementDomain.ComputeMeter(events, entitlementDomain.CurrentSources(sources), latestBudget(budgets), s.clock.Now().UTC())
        return entitlementDomain.DeriveAdmission(state, meter.ConfirmedTotal, latestBudget(budgets)), nil
}

// ExceptionsView lists the operational exceptions with derived acks.
func (s *Service) ExceptionsView(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) (api.ExceptionsView, error) {
        scope, level, err := s.viewLevel(ctx, actor, tenantID)
        if err != nil {
                return api.ExceptionsView{}, err
        }
        exceptions, err := s.repository.ListExceptions(ctx, scope)
        if err != nil {
                return api.ExceptionsView{}, err
        }
        view := api.ExceptionsView{Level: level, Exceptions: exceptions}
        if level == api.LevelSummary {
                view.Exceptions = nil
        }
        return view, nil
}

// ComposeTerms validates and appends the next terms version. Unknown
// mandatory values stay empty and render as open questions.
func (s *Service) ComposeTerms(ctx context.Context, c api.TermsCommand) (entitlementDomain.TermsVersion, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return entitlementDomain.TermsVersion{}, err
        }
        terms := entitlementDomain.TermsVersion{
                Currency:           strings.ToUpper(strings.TrimSpace(c.Currency)),
                TaxRegime:          strings.TrimSpace(c.TaxRegime),
                PeriodStart:        c.PeriodStart,
                PeriodEnd:          c.PeriodEnd,
                IncludedEmployees:  cleanList(c.IncludedEmployees),
                IncludedChannels:   cleanList(c.IncludedChannels),
                IncludedConnectors: cleanList(c.IncludedConnectors),
                ChangeNote:         strings.TrimSpace(c.ChangeNote),
        }
        for _, limit := range c.Limits {
                spec, err := entitlementDomain.LookupCategory(limit.Category)
                if err != nil {
                        return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "limits", Reason: "Категория лимита не найдена в словаре: " + string(limit.Category)}
                }
                if limit.Cap <= 0 {
                        return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "limits", Reason: "Лимит категории " + string(limit.Category) + " должен быть положительным."}
                }
                terms.Limits = append(terms.Limits, entitlementDomain.CategoryLimit{Category: limit.Category, Cap: limit.Cap, Unit: spec.Unit})
        }
        if terms.Currency != "" && !currencyPattern.MatchString(terms.Currency) {
                return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "currency", Reason: "Валюта задаётся тремя заглавными буквами (например RUB)."}
        }
        if len([]rune(terms.TaxRegime)) > 300 {
                return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "tax_regime", Reason: "Налоговый режим: до 300 символов."}
        }
        if terms.PeriodStart != nil && terms.PeriodEnd != nil && terms.PeriodEnd.Before(*terms.PeriodStart) {
                return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "period", Reason: "Конец периода действия не может быть раньше начала."}
        }
        for name, list := range map[string][]string{"included_employees": terms.IncludedEmployees, "included_channels": terms.IncludedChannels, "included_connectors": terms.IncludedConnectors} {
                if err := validateList(list, name); err != nil {
                        return entitlementDomain.TermsVersion{}, err
                }
        }
        if c.LaunchPrice != nil {
                if *c.LaunchPrice < 0 {
                        return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "launch_price", Reason: "Цена запуска не может быть отрицательной."}
                }
                value := round2(*c.LaunchPrice)
                terms.LaunchPrice = &value
        }
        if c.SubscriptionPrice != nil {
                if *c.SubscriptionPrice < 0 {
                        return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "subscription_price", Reason: "Цена подписки не может быть отрицательной."}
                }
                value := round2(*c.SubscriptionPrice)
                terms.SubscriptionPrice = &value
        }
        if len([]rune(terms.ChangeNote)) > 600 {
                return entitlementDomain.TermsVersion{}, entitlementDomain.RuleError{Rule: "change_note", Reason: "Описание изменения: до 600 символов."}
        }
        return s.repository.SaveTerms(ctx, port.TermsParams{Scope: scope, Terms: terms, Now: s.clock.Now().UTC()})
}

// RunStateTransition validates and appends one lifecycle event. Activation
// requires the explicit confirmation and an activation-ready terms version;
// resume records the fresh limits check; end is terminal.
func (s *Service) RunStateTransition(ctx context.Context, c api.StateCommand) (entitlementDomain.StateEvent, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return entitlementDomain.StateEvent{}, err
        }
        switch c.Action {
        case entitlementDomain.EventActivate, entitlementDomain.EventPause, entitlementDomain.EventResume, entitlementDomain.EventEnd:
        default:
                return entitlementDomain.StateEvent{}, entitlementDomain.RuleError{Rule: "action", Reason: "Неизвестное действие жизненного цикла."}
        }
        basis := strings.TrimSpace(c.Basis)
        if basis == "" || len([]rune(basis)) > 600 {
                return entitlementDomain.StateEvent{}, entitlementDomain.RuleError{Rule: "basis", Reason: "Укажите основание решения (до 600 символов)."}
        }
        terms, stateEvents, budgets, sources, events, err := s.loadFacts(ctx, scope)
        if err != nil {
                return entitlementDomain.StateEvent{}, err
        }
        current := entitlementDomain.DeriveEntitlementState(stateEvents)
        if err := entitlementDomain.ValidateTransition(current, c.Action); err != nil {
                return entitlementDomain.StateEvent{}, err
        }
        currentTerms := latestTerms(terms)
        if currentTerms == nil {
                return entitlementDomain.StateEvent{}, entitlementDomain.RuleError{Rule: "terms", Reason: "Составьте версию условий тарифа до изменения коммерческого состояния."}
        }
        now := s.clock.Now().UTC()
        event := entitlementDomain.StateEvent{
                Event:        c.Action,
                TermsVersion: currentTerms.Version,
                Basis:        basis,
                ActorID:      scope.PrincipalID().String(),
                CreatedAt:    now,
        }
        switch c.Action {
        case entitlementDomain.EventActivate:
                if !c.Confirm {
                        return entitlementDomain.StateEvent{}, entitlementDomain.RuleError{Rule: "confirm", Reason: "Подтвердите активацию коммерческих условий."}
                }
                if err := currentTerms.ReadyForActivation(); err != nil {
                        return entitlementDomain.StateEvent{}, err
                }
        case entitlementDomain.EventResume:
                meter := entitlementDomain.ComputeMeter(events, entitlementDomain.CurrentSources(sources), latestBudget(budgets), now)
                event.LimitsCheck = entitlementDomain.LimitsCheckResult(meter.ConfirmedTotal, latestBudget(budgets), now)
        }
        eventID, err := s.ids.New(now)
        if err != nil {
                return entitlementDomain.StateEvent{}, err
        }
        event.EventID = eventID.String()
        if err := s.repository.SaveStateEvent(ctx, port.StateEventParams{Scope: scope, Event: event, Now: now}); err != nil {
                return entitlementDomain.StateEvent{}, err
        }
        return event, nil
}

// SetBudget validates and appends the next budget version: amount,
// currency, period, warning threshold, optional hard limit with a declared
// admission/degraded policy, and quantity anomaly norms.
func (s *Service) SetBudget(ctx context.Context, c api.BudgetCommand) (entitlementDomain.BudgetVersion, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return entitlementDomain.BudgetVersion{}, err
        }
        budget := entitlementDomain.BudgetVersion{
                Amount:         round2(c.Amount),
                Currency:       strings.ToUpper(strings.TrimSpace(c.Currency)),
                PeriodStart:    c.PeriodStart,
                PeriodEnd:      c.PeriodEnd,
                WarningPercent: c.WarningPercent,
                LimitPolicy:    strings.TrimSpace(c.LimitPolicy),
                Note:           strings.TrimSpace(c.Note),
        }
        if budget.Amount <= 0 {
                return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "amount", Reason: "Бюджет должен быть положительной суммой."}
        }
        if !currencyPattern.MatchString(budget.Currency) {
                return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "currency", Reason: "Валюта задаётся тремя заглавными буквами (например RUB)."}
        }
        if budget.PeriodEnd.Before(budget.PeriodStart) {
                return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "period", Reason: "Конец периода измерения не может быть раньше начала."}
        }
        if budget.WarningPercent < 1 || budget.WarningPercent > 100 {
                return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "warning_threshold_percent", Reason: "Порог предупреждения задаётся в процентах от 1 до 100."}
        }
        if !entitlementDomain.LimitPolicies[budget.LimitPolicy] {
                return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "limit_policy", Reason: "Политика hard limit не входит в словарь (block_new_admissions / degraded / notify_only)."}
        }
        if c.HardLimit != nil {
                if *c.HardLimit <= 0 {
                        return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "hard_limit", Reason: "Hard limit должен быть положительной суммой."}
                }
                if *c.HardLimit < budget.Amount {
                        return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "hard_limit", Reason: "Hard limit не может быть меньше суммы бюджета."}
                }
                value := round2(*c.HardLimit)
                budget.HardLimit = &value
        }
        for _, threshold := range c.AnomalyThresholds {
                spec, err := entitlementDomain.LookupCategory(threshold.Category)
                if err != nil {
                        return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "anomaly_thresholds", Reason: "Категория нормы не найдена в словаре: " + string(threshold.Category)}
                }
                if threshold.MaxQuantity <= 0 {
                        return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "anomaly_thresholds", Reason: "Норма категории " + string(threshold.Category) + " должна быть положительной."}
                }
                budget.AnomalyThresholds = append(budget.AnomalyThresholds, entitlementDomain.AnomalyThreshold{Category: threshold.Category, MaxQuantity: threshold.MaxQuantity, Unit: spec.Unit})
        }
        if len([]rune(budget.Note)) > 600 {
                return entitlementDomain.BudgetVersion{}, entitlementDomain.RuleError{Rule: "note", Reason: "Примечание: до 600 символов."}
        }
        saved, err := s.repository.SaveBudget(ctx, port.BudgetParams{Scope: scope, Budget: budget, Now: s.clock.Now().UTC()})
        if err != nil {
                return entitlementDomain.BudgetVersion{}, err
        }
        // The budget declares the thresholds: publishing it over already
        // ingested spend must immediately evaluate the exceptions (R1: they
        // used to appear only at the next ingest). The dedupe key carries the
        // budget version, so a re-published budget re-raises honestly.
        if err := s.evaluateExceptions(ctx, scope, s.clock.Now().UTC()); err != nil {
                return saved, err
        }
        return saved, nil
}

// RegisterSource validates and appends one source registration; the latest
// registration per name wins, so a retired registration removes a source.
func (s *Service) RegisterSource(ctx context.Context, c api.SourceCommand) (entitlementDomain.UsageSource, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return entitlementDomain.UsageSource{}, err
        }
        source := entitlementDomain.UsageSource{
                Name:             strings.TrimSpace(c.Name),
                FreshnessMinutes: c.FreshnessMinutes,
                Retired:          c.Retire,
                Note:             strings.TrimSpace(c.Note),
        }
        if source.Name == "" || len([]rune(source.Name)) > 100 {
                return entitlementDomain.UsageSource{}, entitlementDomain.RuleError{Rule: "name", Reason: "Имя источника: от 1 до 100 символов."}
        }
        if len(c.Categories) == 0 && !source.Retired {
                return entitlementDomain.UsageSource{}, entitlementDomain.RuleError{Rule: "categories", Reason: "Укажите хотя бы одну категорию, которую покрывает источник."}
        }
        for _, category := range c.Categories {
                if _, err := entitlementDomain.LookupCategory(category); err != nil {
                        return entitlementDomain.UsageSource{}, entitlementDomain.RuleError{Rule: "categories", Reason: "Категория не найдена в словаре: " + string(category)}
                }
                source.Categories = append(source.Categories, category)
        }
        if source.FreshnessMinutes < 1 || source.FreshnessMinutes > 100000 {
                return entitlementDomain.UsageSource{}, entitlementDomain.RuleError{Rule: "freshness_minutes", Reason: "Срок свежести задаётся в минутах от 1 до 100000."}
        }
        if len([]rune(source.Note)) > 600 {
                return entitlementDomain.UsageSource{}, entitlementDomain.RuleError{Rule: "note", Reason: "Примечание: до 600 символов."}
        }
        now := s.clock.Now().UTC()
        registrationID, err := s.ids.New(now)
        if err != nil {
                return entitlementDomain.UsageSource{}, err
        }
        source.RegistrationID = registrationID.String()
        return s.repository.SaveSource(ctx, port.SourceParams{Scope: scope, Source: source, Now: now})
}

// IngestUsage validates and appends one usage event. The event is its own
// idempotency record: the same key with the same material payload replays
// the stored result; the same key with a different payload is an honest
// conflict. New events evaluate the budget thresholds, the hard limit and
// the data completeness, raising deduplicated operational exceptions.
func (s *Service) IngestUsage(ctx context.Context, c api.UsageEventCommand) (entitlementDomain.UsageEvent, bool, error) {
        scope, err := s.ingestScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return entitlementDomain.UsageEvent{}, false, err
        }
        key := strings.TrimSpace(c.EventKey)
        if runes := len([]rune(key)); runes < 8 || runes > 200 {
                return entitlementDomain.UsageEvent{}, false, entitlementDomain.ErrInvalidEventKey
        }
        spec, err := entitlementDomain.LookupCategory(c.Category)
        if err != nil {
                return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "category", Reason: "Категория расхода не найдена в словаре."}
        }
        source := strings.TrimSpace(c.Source)
        if source == "" || len([]rune(source)) > 100 {
                return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "source", Reason: "Укажите источник события (до 100 символов)."}
        }
        if c.Quantity <= 0 {
                return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "quantity", Reason: "Количество должно быть положительным."}
        }
        event := entitlementDomain.UsageEvent{
                EventKey:       key,
                Source:         source,
                Category:       c.Category,
                Quantity:       c.Quantity,
                Unit:           spec.Unit,
                Currency:       strings.ToUpper(strings.TrimSpace(c.Currency)),
                Estimate:       c.Estimate,
                PeriodStart:    c.PeriodStart,
                PeriodEnd:      c.PeriodEnd,
                OccurredAt:     c.OccurredAt,
                EmployeeID:     strings.TrimSpace(c.EmployeeID),
                Channel:        strings.TrimSpace(c.Channel),
                ReleaseVersion: strings.TrimSpace(c.ReleaseVersion),
                Provenance:     strings.TrimSpace(c.Provenance),
                ActorID:        scope.PrincipalID().String(),
        }
        if event.PeriodEnd.Before(event.PeriodStart) {
                return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "period", Reason: "Конец периода события не может быть раньше начала."}
        }
        if event.OccurredAt.IsZero() {
                event.OccurredAt = s.clock.Now().UTC()
        }
        if c.Amount != nil {
                if *c.Amount < 0 {
                        return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "amount", Reason: "Сумма не может быть отрицательной."}
                }
                value := round2(*c.Amount)
                event.Amount = &value
        }
        if spec.Money && event.Amount == nil {
                return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "amount", Reason: "Категория " + string(spec.ID) + " учитывает плату: укажите сумму и валюту."}
        }
        if event.Amount != nil && !currencyPattern.MatchString(event.Currency) {
                return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: "currency", Reason: "Валюта задаётся тремя заглавными буквами (например RUB)."}
        }
        for _, field := range []struct {
            name  string
            value string
            max   int
        }{
            {"employee_id", event.EmployeeID, 100},
            {"channel", event.Channel, 100},
            {"release_version", event.ReleaseVersion, 100},
            {"provenance", event.Provenance, 600},
        } {
                if len([]rune(field.value)) > field.max {
                        return entitlementDomain.UsageEvent{}, false, entitlementDomain.RuleError{Rule: field.name, Reason: "Поле превышает допустимую длину."}
                }
        }
        now := s.clock.Now().UTC()
        eventID, err := s.ids.New(now)
        if err != nil {
                return entitlementDomain.UsageEvent{}, false, err
        }
        event.EventID = eventID.String()
        event.IngestedAt = now
        event.AggRuleVersion = entitlementDomain.AggregationRuleVersion
        event.PayloadHash = usagePayloadHash(scope.TenantID(), event)
        stored, replayed, err := s.repository.IngestUsage(ctx, port.IngestParams{Scope: scope, Event: event, Now: now})
        if err != nil {
                return entitlementDomain.UsageEvent{}, false, err
        }
        if !replayed {
                if err := s.evaluateExceptions(ctx, scope, now); err != nil {
                        return stored, false, err
                }
        }
        return stored, replayed, nil
}

// evaluateExceptions computes the current meter and raises the deduplicated
// operational exceptions: the warning threshold, the hard limit, the data
// gaps and the quantity anomaly norms. Messages never claim an exact amount
// while the data is incomplete.
func (s *Service) evaluateExceptions(ctx context.Context, scope tenancy.Context, now time.Time) error {
        _, _, budgets, sources, events, err := s.loadFacts(ctx, scope)
        if err != nil {
                return err
        }
        budget := latestBudget(budgets)
        meter := entitlementDomain.ComputeMeter(events, entitlementDomain.CurrentSources(sources), budget, now)
        // The dedupe key names the period and the budget version (the note of
        // the spec): a re-published budget re-discovers the same facts under a
        // new version instead of silently suppressing them.
        periodKey := "no-budget"
        if budget != nil {
                periodKey = fmt.Sprintf("v%d:%s:%s", budget.Version, budget.PeriodStart.Format("2006-01-02"), budget.PeriodEnd.Format("2006-01-02"))
        }
        if meter.DataStatus != entitlementDomain.DataComplete {
                if err := s.raiseException(ctx, scope, entitlementDomain.UsageException{
                        Cause:             entitlementDomain.CauseDataGap,
                        Message:           dataGapMessage(meter),
                        DataStatus:        meter.DataStatus,
                        RecommendedAction: "Проверьте доставку событий от источников; неизвестный расход не считается нулём.",
                        DedupeKey:          "gap:" + periodKey,
                        AggRuleVersion:    entitlementDomain.AggregationRuleVersion,
                        ActorID:           scope.PrincipalID().String(),
                        CreatedAt:         now,
                }); err != nil {
                        return err
                }
        }
        if budget == nil {
                return nil
        }
        if meter.Budget != nil && meter.Budget.Warned {
            if err := s.raiseException(ctx, scope, entitlementDomain.UsageException{
                Cause:             entitlementDomain.CauseBudgetThreshold,
                Message:           thresholdMessage(meter, budget),
                DataStatus:        meter.DataStatus,
                RecommendedAction: "Проверьте прогноз расхода и объём работы до конца периода.",
                DedupeKey:          "warn:" + periodKey,
                AggRuleVersion:    entitlementDomain.AggregationRuleVersion,
                ActorID:           scope.PrincipalID().String(),
                CreatedAt:         now,
            }); err != nil {
                return err
            }
        }
        if entitlementDomain.HardLimitReached(meter.ConfirmedTotal, budget) {
            if err := s.raiseException(ctx, scope, entitlementDomain.UsageException{
                Cause:             entitlementDomain.CauseHardLimit,
                Message:           hardLimitMessage(meter, budget),
                DataStatus:        meter.DataStatus,
                RecommendedAction: "Применена объявленная политика " + budget.LimitPolicy + ": новые необязательные операции остановлены; safety, handoff и принятые обязательства продолжаются. Ответственный: Owner.",
                DedupeKey:          "limit:" + periodKey,
                AggRuleVersion:    entitlementDomain.AggregationRuleVersion,
                ActorID:           scope.PrincipalID().String(),
                CreatedAt:         now,
            }); err != nil {
                return err
            }
        }
        for _, threshold := range budget.AnomalyThresholds {
            for _, row := range meter.Categories {
                if row.Category != threshold.Category || row.Quantity <= threshold.MaxQuantity {
                        continue
                }
                cause := entitlementDomain.CauseAnomalyTools
                if threshold.Category == entitlementDomain.CategoryManualMinutes {
                        cause = entitlementDomain.CauseAnomalyManual
                }
                if err := s.raiseException(ctx, scope, entitlementDomain.UsageException{
                    Cause:             cause,
                    Message:           anomalyMessage(row, threshold),
                    DataStatus:        meter.DataStatus,
                    RecommendedAction: "Проверьте маршрут работы ИИ-сотрудника: количество превышает заявленную норму периода.",
                    DedupeKey:          fmt.Sprintf("anomaly:%s:%s", periodKey, threshold.Category),
                    AggRuleVersion:    entitlementDomain.AggregationRuleVersion,
                    ActorID:           scope.PrincipalID().String(),
                    CreatedAt:         now,
                }); err != nil {
                    return err
                }
            }
        }
        return nil
}

// amountOrHonesty renders the confirmed amount only when the data is
// complete; incomplete data never claims an exact sum.
func amountOrHonesty(meter entitlementDomain.Meter, currency string) string {
        if meter.DataStatus != entitlementDomain.DataComplete {
                return "данные за период неполные (статус: " + meter.DataStatus + "), точная сумма не утверждается"
        }
        return fmt.Sprintf("подтверждённый расход: %.2f %s", meter.ConfirmedTotal[currency], currency)
}

func thresholdMessage(meter entitlementDomain.Meter, budget *entitlementDomain.BudgetVersion) string {
        return "Расход приближается к порогу предупреждения (" + strconv.Itoa(budget.WarningPercent) + "% бюджета " + fmt.Sprintf("%.2f", budget.Amount) + " " + budget.Currency + "); " + amountOrHonesty(meter, budget.Currency) + "."
}

func hardLimitMessage(meter entitlementDomain.Meter, budget *entitlementDomain.BudgetVersion) string {
        return "Подтверждённый расход достиг hard limit (" + fmt.Sprintf("%.2f", *budget.HardLimit) + " " + budget.Currency + "); " + amountOrHonesty(meter, budget.Currency) + ". Оценки (estimate) в решении не участвуют."
}

func anomalyMessage(row entitlementDomain.CategoryMeter, threshold entitlementDomain.AnomalyThreshold) string {
        return fmt.Sprintf("Аномальное количество по категории %s: %.2f %s при норме %.2f %s за период.", row.Title, row.Quantity, row.Unit, threshold.MaxQuantity, threshold.Unit)
}

func dataGapMessage(meter entitlementDomain.Meter) string {
        if len(meter.MissingCategories) > 0 {
                return "Отсутствуют источники для категорий: " + strings.Join(meter.MissingCategories, ", ") + "."
        }
        return "Полнота данных: " + meter.DataStatus + ". Неизвестный расход не заменяется нулём."
}

func (s *Service) raiseException(ctx context.Context, scope tenancy.Context, exception entitlementDomain.UsageException) error {
        if exception.ExceptionID == "" {
                exceptionID, err := s.ids.New(exception.CreatedAt)
                if err != nil {
                        return err
                }
                exception.ExceptionID = exceptionID.String()
        }
        _, _, err := s.repository.SaveException(ctx, port.ExceptionParams{Scope: scope, Exception: exception, Now: exception.CreatedAt})
        return err
}

// RaiseException records a manual operational exception: the cause comes
// from the honest dictionary (including anomaly_retry_storm, whose automatic
// trigger needs runtime signals of Epic 2).
func (s *Service) RaiseException(ctx context.Context, c api.ManualExceptionCommand) (entitlementDomain.UsageException, error) {
        scope, err := s.manageScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return entitlementDomain.UsageException{}, err
        }
        if !entitlementDomain.ExceptionCauses[c.Cause] {
                return entitlementDomain.UsageException{}, entitlementDomain.RuleError{Rule: "cause", Reason: "Причина исключения не входит в словарь."}
        }
        message := strings.TrimSpace(c.Message)
        if message == "" || len([]rune(message)) > 1000 {
                return entitlementDomain.UsageException{}, entitlementDomain.RuleError{Rule: "message", Reason: "Опишите наблюдение (до 1000 символов)."}
        }
        action := strings.TrimSpace(c.RecommendedAction)
        if action == "" || len([]rune(action)) > 600 {
                return entitlementDomain.UsageException{}, entitlementDomain.RuleError{Rule: "recommended_action", Reason: "Укажите рекомендуемое действие (до 600 символов)."}
        }
        terms, stateEvents, budgets, sources, events, err := s.loadFacts(ctx, scope)
        if err != nil {
                return entitlementDomain.UsageException{}, err
        }
        _ = terms
        _ = stateEvents
        now := s.clock.Now().UTC()
        meter := entitlementDomain.ComputeMeter(events, entitlementDomain.CurrentSources(sources), latestBudget(budgets), now)
        exceptionID, err := s.ids.New(now)
        if err != nil {
                return entitlementDomain.UsageException{}, err
        }
        exception := entitlementDomain.UsageException{
                ExceptionID:       exceptionID.String(),
                Cause:             c.Cause,
                Message:           message,
                DataStatus:        meter.DataStatus,
                AffectedEmployee:  strings.TrimSpace(c.AffectedEmployee),
                AffectedRoute:     strings.TrimSpace(c.AffectedRoute),
                RecommendedAction: action,
                DedupeKey:         "manual:" + exceptionID.String(),
                AggRuleVersion:    entitlementDomain.AggregationRuleVersion,
                ActorID:           scope.PrincipalID().String(),
                CreatedAt:         now,
        }
        _, _, err = s.repository.SaveException(ctx, port.ExceptionParams{Scope: scope, Exception: exception, Now: now})
        return exception, err
}

// AcknowledgeException appends the acknowledgement of one exception; the
// derived ack state is the latest acknowledgement event.
func (s *Service) AcknowledgeException(ctx context.Context, c api.AckCommand) error {
        scope, err := s.ingestScope(ctx, c.Actor, c.TenantID)
        if err != nil {
                return err
        }
        if !c.ExceptionID.Valid() {
                return entitlementDomain.ErrNotFound
        }
        note := strings.TrimSpace(c.Note)
        if len([]rune(note)) > 600 {
                return entitlementDomain.RuleError{Rule: "note", Reason: "Заметка подтверждения: до 600 символов."}
        }
        exceptions, err := s.repository.ListExceptions(ctx, scope)
        if err != nil {
                return err
        }
        found := false
        for _, exception := range exceptions {
                if exception.ExceptionID == c.ExceptionID.String() {
                        found = true
                        break
                }
        }
        if !found {
                return entitlementDomain.ErrNotFound
        }
        now := s.clock.Now().UTC()
        ackID, err := s.ids.New(now)
        if err != nil {
                return err
        }
        return s.repository.SaveAck(ctx, port.AckParams{
                Scope: scope,
                Ack: entitlementDomain.ExceptionAck{
                        ExceptionID: c.ExceptionID.String(),
                        AckID:       ackID.String(),
                        ActorID:     scope.PrincipalID().String(),
                        Note:        note,
                        CreatedAt:   now,
                },
                Now: now,
        })
}

func cleanList(values []string) []string {
        out := []string{}
        for _, value := range values {
                trimmed := strings.TrimSpace(value)
                if trimmed != "" {
                        out = append(out, trimmed)
                }
        }
        return out
}

func validateList(list []string, name string) error {
        if len(list) > 50 {
                return entitlementDomain.RuleError{Rule: name, Reason: "Список слишком длинный (до 50 элементов)."}
        }
        for _, value := range list {
                if len([]rune(value)) > 300 {
                        return entitlementDomain.RuleError{Rule: name, Reason: "Элемент списка превышает 300 символов."}
                }
        }
        return nil
}

func round2(value float64) float64 {
        return float64(int64(value*100+0.5)) / 100
}

// usagePayloadHash canonicalizes the material payload of one usage event:
// the same key with a different hash is an honest conflict for review.
func usagePayloadHash(tenant interface{ String() string }, event entitlementDomain.UsageEvent) string {
        h := sha256.New()
        _, _ = h.Write([]byte("entitlement.usage.v1"))
        _, _ = h.Write([]byte{0})
        _, _ = h.Write([]byte(tenant.String()))
        writeField := func(value string) {
                _, _ = h.Write([]byte{0})
                _, _ = h.Write([]byte(value))
        }
        writeField(event.EventKey)
        writeField(event.Source)
        writeField(string(event.Category))
        writeField(strconv.FormatFloat(event.Quantity, 'f', 4, 64))
        if event.Amount != nil {
                writeField(strconv.FormatFloat(*event.Amount, 'f', 2, 64))
        } else {
                writeField("-")
        }
        writeField(event.Currency)
        writeField(strconv.FormatBool(event.Estimate))
        writeField(event.PeriodStart.Format(time.RFC3339))
        writeField(event.PeriodEnd.Format(time.RFC3339))
        writeField(event.OccurredAt.Format(time.RFC3339))
        writeField(event.EmployeeID)
        writeField(event.Channel)
        writeField(event.ReleaseVersion)
        writeField(event.Provenance)
        return hex.EncodeToString(h.Sum(nil))
}

// AdmissionFacts is the caller-authority admission read (the
// ConnectionsFacts/MembersFacts precedent): the consumer (the employee
// RouteFacts facade relaying the word for the Story 2-1 ingress) has
// already verified its own scope from the trusted connection metadata, so
// this read does not re-require a user principal; RLS still isolates the
// tenant. The concrete derivation is the same DeriveAdmission of the
// acting state, the meter and the budget — never a copied "ok".
func (s *Service) AdmissionFacts(ctx context.Context, scope tenancy.Context) (entitlementDomain.AdmissionState, error) {
	if !scope.Valid() {
		return entitlementDomain.AdmissionState{}, entitlementDomain.ErrNotFound
	}
	_, stateEvents, budgets, sources, events, err := s.loadFacts(ctx, scope)
	if err != nil {
		return entitlementDomain.AdmissionState{}, err
	}
	state := entitlementDomain.DeriveEntitlementState(stateEvents)
	meter := entitlementDomain.ComputeMeter(events, entitlementDomain.CurrentSources(sources), latestBudget(budgets), s.clock.Now().UTC())
	return entitlementDomain.DeriveAdmission(state, meter.ConfirmedTotal, latestBudget(budgets)), nil
}
