package api

import (
        "context"
        "time"

        "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        entitlementDomain "github.com/artemiosu/staffy/internal/entitlement/domain"
)

// Clock and IDGenerator follow the module conventions of identity/connector.
type Clock interface{ Now() time.Time }

type IDGenerator interface {
        New(time.Time) (id.UUID, error)
}

// Detail levels of the commercial surfaces: a member without the commercial
// or audit scope sees only the summary level (state and qualitative budget
// status), never sums, terms or breakdowns.
const (
        LevelFull    = "full"
        LevelSummary = "summary"
)

// TermsCommand publishes a new append-only version of the commercial terms.
// Unknown mandatory values stay empty: the draft view renders them as open
// questions, never as zero cost.
type TermsCommand struct {
        Actor             domain.Principal
        TenantID          id.UUID
        Currency          string
        TaxRegime         string
        PeriodStart       *time.Time
        PeriodEnd         *time.Time
        IncludedEmployees  []string
        IncludedChannels   []string
        IncludedConnectors []string
        Limits            []entitlementDomain.CategoryLimit
        LaunchPrice        *float64
        SubscriptionPrice  *float64
        ChangeNote        string
        CorrelationID     id.UUID
}

// StateCommand runs one entitlement lifecycle transition: activate requires
// an explicit confirmation; pause/end require a reason; resume records a
// fresh limits check.
type StateCommand struct {
        Actor         domain.Principal
        TenantID      id.UUID
        Action        string // activate / pause / resume / end
        Basis         string
        Confirm       bool
        CorrelationID id.UUID
}

// BudgetCommand publishes a new append-only budget version.
type BudgetCommand struct {
        Actor             domain.Principal
        TenantID          id.UUID
        Amount            float64
        Currency          string
        PeriodStart       time.Time
        PeriodEnd         time.Time
        WarningPercent    int
        HardLimit         *float64
        LimitPolicy       string
        AnomalyThresholds []entitlementDomain.AnomalyThreshold
        Note              string
        CorrelationID     id.UUID
}

// SourceCommand registers (or retires) one usage source.
type SourceCommand struct {
        Actor            domain.Principal
        TenantID         id.UUID
        Name             string
        Categories       []entitlementDomain.UsageCategory
        FreshnessMinutes int
        Retire           bool
        Note             string
        CorrelationID    id.UUID
}

// UsageEventCommand ingests one usage event with a tenant-scoped key.
type UsageEventCommand struct {
        Actor           domain.Principal
        TenantID        id.UUID
        EventKey        string
        Source          string
        Category        entitlementDomain.UsageCategory
        Quantity        float64
        Amount          *float64
        Currency        string
        Estimate        bool
        PeriodStart     time.Time
        PeriodEnd       time.Time
        OccurredAt      time.Time
        EmployeeID      string
        Channel         string
        ReleaseVersion  string
        Provenance      string
        CorrelationID   id.UUID
}

// ManualExceptionCommand records one operational exception by hand (for
// example an anomaly the automatic thresholds cannot see yet).
type ManualExceptionCommand struct {
        Actor             domain.Principal
        TenantID          id.UUID
        Cause             string
        AffectedEmployee  string
        AffectedRoute     string
        RecommendedAction string
        Message           string
        CorrelationID     id.UUID
}

// AckCommand acknowledges one exception.
type AckCommand struct {
        Actor         domain.Principal
        TenantID      id.UUID
        ExceptionID   id.UUID
        Note          string
        CorrelationID id.UUID
}

// EntitlementView is the derived projection of the commercial perimeter:
// the current state, the composition skeleton, the current terms version
// with its open questions, the version history and the admission state.
// SummaryLevel views carry only the state and the qualitative budget
// status.
type EntitlementView struct {
        Level          string                            `json:"level"`
        State          string                            `json:"state"`
        Composition    []entitlementDomain.CategorySpec  `json:"composition"`
        Terms          *entitlementDomain.TermsVersion   `json:"terms,omitempty"`
        OpenQuestions  []string                          `json:"open_questions"`
        History        []entitlementDomain.TermsVersion  `json:"history"`
        StateEvents    []entitlementDomain.StateEvent    `json:"state_events"`
        Budget         *entitlementDomain.BudgetVersion  `json:"budget,omitempty"`
        BudgetQuality  string                            `json:"budget_quality"` // summary level only
        Admission      entitlementDomain.AdmissionState  `json:"admission"`
        Sources        []entitlementDomain.UsageSource   `json:"sources"`
}

// MeterView is the derived Budget Meter. SummaryLevel views carry only the
// data status and the qualitative budget relation, never sums.
type MeterView struct {
        Level          string                        `json:"level"`
        DataStatus     string                        `json:"data_status"`
        PeriodStart    *time.Time                    `json:"period_start,omitempty"`
        PeriodEnd      *time.Time                    `json:"period_end,omitempty"`
        LastIngestedAt *time.Time                    `json:"last_ingested_at,omitempty"`
        AggRuleVersion string                        `json:"agg_rule_version"`
        Categories     []entitlementDomain.CategoryMeter `json:"categories"`
        MissingCategories []string                   `json:"missing_categories"`
        ConfirmedTotal map[string]float64            `json:"confirmed_total"`
        EstimateTotal  map[string]float64            `json:"estimate_total"`
        Budget         *entitlementDomain.BudgetView `json:"budget,omitempty"`
        BudgetQuality  string                        `json:"budget_quality"` // summary level only
        Breakdowns     []entitlementDomain.BreakdownRow `json:"breakdowns"`
}

// ExceptionsView lists the exceptions with their derived acknowledgements.
type ExceptionsView struct {
        Level      string                              `json:"level"`
        Exceptions []entitlementDomain.UsageException  `json:"exceptions"`
}

// Service is the application surface consumed by the transport layer.
type Service interface {
        EntitlementView(context.Context, domain.Principal, id.UUID) (EntitlementView, error)
        MeterView(context.Context, domain.Principal, id.UUID) (MeterView, error)
        AdmissionView(context.Context, domain.Principal, id.UUID) (entitlementDomain.AdmissionState, error)
        ExceptionsView(context.Context, domain.Principal, id.UUID) (ExceptionsView, error)
        ComposeTerms(context.Context, TermsCommand) (entitlementDomain.TermsVersion, error)
        RunStateTransition(context.Context, StateCommand) (entitlementDomain.StateEvent, error)
        SetBudget(context.Context, BudgetCommand) (entitlementDomain.BudgetVersion, error)
        RegisterSource(context.Context, SourceCommand) (entitlementDomain.UsageSource, error)
        IngestUsage(context.Context, UsageEventCommand) (entitlementDomain.UsageEvent, bool, error)
        RaiseException(context.Context, ManualExceptionCommand) (entitlementDomain.UsageException, error)
        AcknowledgeException(context.Context, AckCommand) error
}
