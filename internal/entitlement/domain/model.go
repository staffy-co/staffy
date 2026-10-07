package domain

import (
        "errors"
        "time"
)

// Errors of the entitlement module. Authorization failures surface as
// ErrNotFound from the application layer (tenant isolation is not disclosed).
var (
        ErrNotFound         = errors.New("entitlement resource not found")
        ErrForbidden        = errors.New("principal is not allowed to manage the commercial perimeter")
        ErrInvalidEventKey  = errors.New("usage event key is invalid")
        ErrEventKeyConflict = errors.New("usage event key conflicts with another payload")
        ErrUnknownCategory  = errors.New("usage category is not in the system dictionary")
)

// RuleError is a named validation rejection with an honest human reason.
type RuleError struct {
        Rule   string
        Reason string
}

func (e RuleError) Error() string { return e.Reason }

// Entitlement states, derived from append-only state events. There is no
// mutable current-state table: the state is a pure function of the events.
const (
        StateDraft   = "draft"
        StateActive  = "active"
        StatePaused  = "paused"
        StateEnded   = "ended"
)

// State events of the entitlement lifecycle. `activate` requires an explicit
// confirmation; `resume` carries a fresh limits check; `end` is terminal.
const (
        EventActivate = "activate"
        EventPause    = "pause"
        EventResume   = "resume"
        EventEnd      = "end"
)

// Data completeness statuses of the Budget Meter. Unknown spend is never
// replaced by zero: `unknown` is a honest statement, not an empty sum.
const (
        DataComplete = "complete"
        DataPartial  = "partial"
        DataStale    = "stale"
        DataUnknown  = "unknown"
)

// Hard limit policies, declared up front with the budget. What keeps working
// under every policy is fixed by ContinuesList — never silently cut.
const (
        PolicyBlockNew = "block_new_admissions"
        PolicyDegraded = "degraded"
        PolicyNotify   = "notify_only"
)

// LimitPolicies is the allowed dictionary of admission/degraded outcomes.
var LimitPolicies = map[string]bool{
        PolicyBlockNew: true,
        PolicyDegraded: true,
        PolicyNotify:   true,
}

// Exception causes: the honest dictionary. anomaly_retry_storm exists in the
// dictionary from day one; its automatic trigger needs runtime signals
// (Epic 2), so in this slice it is raised by manual or ingested events only.
const (
        CauseBudgetThreshold   = "budget_threshold"
        CauseHardLimit         = "hard_limit_reached"
        CauseDataGap           = "data_gap"
        CauseAnomalyTools      = "anomaly_tool_calls"
        CauseAnomalyRetryStorm = "anomaly_retry_storm"
        CauseAnomalyManual     = "anomaly_manual_minutes"
        CauseManualEntry       = "manual_entry"
)

// ExceptionCauses is the allowed dictionary of exception causes.
var ExceptionCauses = map[string]bool{
        CauseBudgetThreshold:   true,
        CauseHardLimit:         true,
        CauseDataGap:           true,
        CauseAnomalyTools:      true,
        CauseAnomalyRetryStorm: true,
        CauseAnomalyManual:     true,
        CauseManualEntry:       true,
}

// AggregationRuleVersion stamps every usage event at ingest time and every
// recalculated aggregate: the meter always names the rules it used.
const AggregationRuleVersion = "entitlement.meter.v1"

// Admission states: the derived answer for future consumers (readiness 1-14,
// Epic 2) of whether new admissions/publications may run.
const (
        AdmissionOK       = "ok"
        AdmissionBlocked  = "blocked_new_admissions"
        AdmissionDegraded = "degraded"
)

// ContinuesList names what keeps working under every admission policy:
// safety, human handoff, reconciliation and accepted commitments are never
// silently cut by a hard limit, pause or end.
var ContinuesList = []string{
        "безопасность участников (safety)",
        "передача человеку (human handoff)",
        "сверка и восстановление (reconciliation)",
        "уже принятые обязательства",
}

// AdmissionOwner is the responsible party for the next step of every blocked
// or degraded admission state.
const AdmissionOwner = "Owner"

// CategoryLimit is one declared usage limit of the terms version.
type CategoryLimit struct {
        Category UsageCategory `json:"category"`
        Cap      float64       `json:"cap"`
        Unit     string        `json:"unit"`
}

// TermsVersion is one append-only version of the commercial terms. Unknown
// mandatory values stay empty/nil: the draft view renders them as open
// questions, never as zero cost. A material change creates a new version;
// the past is never rewritten and past usage is never recalculated.
type TermsVersion struct {
        TenantID           string          `json:"tenant_id"`
        Version            int64           `json:"version"`
        Currency           string          `json:"currency"`
        TaxRegime          string          `json:"tax_regime"`
        PeriodStart        *time.Time      `json:"period_start,omitempty"`
        PeriodEnd          *time.Time      `json:"period_end,omitempty"`
        IncludedEmployees  []string        `json:"included_employees"`
        IncludedChannels   []string        `json:"included_channels"`
        IncludedConnectors []string        `json:"included_connectors"`
        Limits             []CategoryLimit `json:"limits"`
        LaunchPrice        *float64        `json:"launch_price,omitempty"`
        SubscriptionPrice  *float64        `json:"subscription_price,omitempty"`
        ChangeNote         string          `json:"change_note"`
        ActorID            string          `json:"actor_id"`
        CreatedAt          time.Time       `json:"created_at"`
}

// OpenQuestions lists the honest open questions of this version: every
// unknown mandatory commercial attribute is named, not guessed.
func (t TermsVersion) OpenQuestions() []string {
        missing := []string{}
        if t.Currency == "" {
                missing = append(missing, "валюта не определена")
        }
        if t.TaxRegime == "" {
                missing = append(missing, "налоговый режим не определён")
        }
        if t.PeriodStart == nil {
                missing = append(missing, "начало периода действия не определено")
        }
        if t.PeriodEnd == nil {
                missing = append(missing, "конец периода действия не определён")
        }
        if t.LaunchPrice == nil {
                missing = append(missing, "цена запуска не определена (не ноль, а открытый вопрос)")
        }
        if t.SubscriptionPrice == nil {
                missing = append(missing, "цена подписки ИИ-сотрудника не определена (не ноль, а открытый вопрос)")
        }
        if len(t.IncludedEmployees) == 0 {
                missing = append(missing, "включённые ИИ-сотрудники не определены")
        }
        if len(t.IncludedChannels) == 0 {
                missing = append(missing, "включённые каналы не определены (не показываются как включённые)")
        }
        if len(t.IncludedConnectors) == 0 {
                missing = append(missing, "включённые Connectors не определены (не показываются как включённые)")
        }
        if len(t.Limits) == 0 {
                missing = append(missing, "лимиты не определены (не показываются как включённые)")
        }
        return missing
}

// ReadyForActivation reports whether the version carries the attributes
// activation honestly requires: currency and period start. Everything else
// may stay an open question — but never a silent zero.
func (t TermsVersion) ReadyForActivation() error {
        if t.Currency == "" {
                return RuleError{Rule: "currency", Reason: "Для активации определите валюту условий."}
        }
        if t.PeriodStart == nil {
                return RuleError{Rule: "period_start", Reason: "Для активации определите начало периода действия."}
        }
        return nil
}

// StateEvent is one append-only entitlement lifecycle event. The event fixes
// who confirmed the commercial decision, when and on what basis (FR-85).
type StateEvent struct {
        TenantID     string    `json:"tenant_id"`
        EventID      string    `json:"event_id"`
        Event        string    `json:"event"`
        TermsVersion int64     `json:"terms_version"`
        Basis        string    `json:"basis"`
        LimitsCheck  string    `json:"limits_check,omitempty"`
        ActorID      string    `json:"actor_id"`
        CreatedAt    time.Time `json:"created_at"`
}

// AnomalyThreshold is one declared quantity norm of the budget: crossing it
// raises an operational exception (tool calls, manual minutes).
type AnomalyThreshold struct {
        Category    UsageCategory `json:"category"`
        MaxQuantity float64       `json:"max_quantity"`
        Unit        string        `json:"unit"`
}

// BudgetVersion is one append-only version of the pilot budget: amount,
// currency, measurement period, warning threshold, optional hard limit with
// a declared admission/degraded policy, and quantity anomaly norms.
type BudgetVersion struct {
        TenantID          string             `json:"tenant_id"`
        Version           int64              `json:"version"`
        Amount            float64            `json:"amount"`
        Currency          string             `json:"currency"`
        PeriodStart       time.Time          `json:"period_start"`
        PeriodEnd         time.Time          `json:"period_end"`
        WarningPercent    int                `json:"warning_threshold_percent"`
        HardLimit         *float64           `json:"hard_limit,omitempty"`
        LimitPolicy       string             `json:"limit_policy"`
        AnomalyThresholds []AnomalyThreshold `json:"anomaly_thresholds"`
        Note              string             `json:"note"`
        ActorID           string             `json:"actor_id"`
        CreatedAt         time.Time          `json:"created_at"`
}

// UsageSource is one append-only source registration: which categories the
// source is expected to cover and how fresh its events must stay. The
// current set of sources is the latest registration per name.
type UsageSource struct {
        TenantID         string           `json:"tenant_id"`
        RegistrationID   string           `json:"registration_id"`
        Name             string           `json:"name"`
        Categories       []UsageCategory  `json:"categories"`
        FreshnessMinutes int              `json:"freshness_minutes"`
        Retired          bool             `json:"retired"`
        Note             string           `json:"note"`
        ActorID          string           `json:"actor_id"`
        CreatedAt        time.Time        `json:"created_at"`
}

// CurrentSources derives the active source set: the latest registration per
// name wins; a retired latest registration removes the source.
func CurrentSources(all []UsageSource) map[string]UsageSource {
        latest := map[string]UsageSource{}
        for _, source := range all {
                current, seen := latest[source.Name]
                if !seen || source.CreatedAt.After(current.CreatedAt) ||
                        (source.CreatedAt.Equal(current.CreatedAt) && source.RegistrationID > current.RegistrationID) {
                        latest[source.Name] = source
                }
        }
        active := map[string]UsageSource{}
        for name, source := range latest {
                if !source.Retired {
                        active[name] = source
                }
        }
        return active
}

// UsageEvent is one ingested usage record. The event itself is the
// idempotency record: UNIQUE (tenant, event_key) plus the payload hash
// decide replay or conflict. Events carry identifiers, counters and amounts
// only — never conversation content.
type UsageEvent struct {
        TenantID       string        `json:"tenant_id"`
        EventID        string        `json:"event_id"`
        EventKey       string        `json:"event_key"`
        PayloadHash    string        `json:"payload_hash"`
        Source         string        `json:"source"`
        Category       UsageCategory `json:"category"`
        Quantity       float64       `json:"quantity"`
        Unit           string        `json:"unit"`
        Amount         *float64      `json:"amount,omitempty"`
        Currency       string        `json:"currency"`
        Estimate       bool          `json:"is_estimate"`
        PeriodStart    time.Time     `json:"period_start"`
        PeriodEnd      time.Time     `json:"period_end"`
        OccurredAt     time.Time     `json:"occurred_at"`
        IngestedAt     time.Time     `json:"ingested_at"`
        EmployeeID     string        `json:"employee_id,omitempty"`
        Channel        string        `json:"channel,omitempty"`
        ReleaseVersion string        `json:"release_version,omitempty"`
        Provenance     string        `json:"provenance"`
        AggRuleVersion string        `json:"agg_rule_version"`
        ActorID        string        `json:"actor_id"`
}

// ExceptionAck is one append-only acknowledgement of an exception.
type ExceptionAck struct {
        TenantID    string    `json:"tenant_id"`
        ExceptionID string    `json:"exception_id"`
        AckID       string    `json:"ack_id"`
        ActorID     string    `json:"actor_id"`
        Note        string    `json:"note"`
        CreatedAt   time.Time `json:"created_at"`
}

// UsageException is one operational exception: cause from the dictionary,
// honest message (no exact amount when the data was incomplete), affected
// employee/route, recommended action and the derived acknowledgement.
type UsageException struct {
        TenantID          string        `json:"tenant_id"`
        ExceptionID       string        `json:"exception_id"`
        Cause             string        `json:"cause"`
        Message           string        `json:"message"`
        DataStatus        string        `json:"data_status"`
        AffectedEmployee  string        `json:"affected_employee,omitempty"`
        AffectedRoute     string        `json:"affected_route,omitempty"`
        RecommendedAction string        `json:"recommended_action"`
        DedupeKey         string        `json:"dedupe_key"`
        AggRuleVersion    string        `json:"agg_rule_version"`
        ActorID           string        `json:"actor_id"`
        CreatedAt         time.Time     `json:"created_at"`
        Acknowledged      *ExceptionAck `json:"acknowledged,omitempty"`
}
