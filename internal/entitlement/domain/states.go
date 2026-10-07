package domain

import (
        "sort"
        "time"
)

// DeriveEntitlementState folds the append-only state events into the current
// entitlement state. `end` is terminal: later events cannot resurrect an
// ended entitlement. The fold is deterministic: events are ordered by
// creation time with the event id as the tie-breaker.
func DeriveEntitlementState(events []StateEvent) string {
        ordered := append([]StateEvent(nil), events...)
        sort.Slice(ordered, func(i, j int) bool {
                if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
                        return ordered[i].EventID < ordered[j].EventID
                }
                return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
        })
        state := StateDraft
        for _, event := range ordered {
                switch state {
                case StateEnded:
                        return StateEnded
                }
                switch event.Event {
                case EventActivate:
                        if state == StateDraft {
                                state = StateActive
                        }
                case EventResume:
                        if state == StatePaused {
                                state = StateActive
                        }
                case EventPause:
                        if state == StateActive {
                                state = StatePaused
                        }
                case EventEnd:
                        if state == StateActive || state == StatePaused {
                                state = StateEnded
                        }
                }
        }
        return state
}

// ValidateTransition checks whether a lifecycle command is applicable in the
// current state, with an honest reason when it is not.
func ValidateTransition(current, next string) error {
        switch next {
        case EventActivate:
                if current != StateDraft {
                        return RuleError{Rule: "state", Reason: "Активация возможна только из состояния draft (для возобновления используйте resume)."}
                }
        case EventPause:
                if current != StateActive {
                        return RuleError{Rule: "state", Reason: "Приостановка возможна только из состояния active."}
                }
        case EventResume:
                if current != StatePaused {
                        return RuleError{Rule: "state", Reason: "Возобновление возможно только из состояния paused."}
                }
        case EventEnd:
                if current != StateActive && current != StatePaused {
                        return RuleError{Rule: "state", Reason: "Завершение возможно из состояний active или paused."}
                }
        default:
                return RuleError{Rule: "event", Reason: "Неизвестное событие жизненного цикла."}
        }
        return nil
}

// AdmissionState is the derived admission answer for future consumers
// (readiness 1-14, Epic 2). Enforcement is not wired in this slice: the
// state is a pure function of the entitlement state, the confirmed spend
// and the declared budget policy.
type AdmissionState struct {
        Status    string   `json:"status"`
        Reason    string   `json:"reason"`
        Policy    string   `json:"policy,omitempty"`
        Continues []string `json:"continues,omitempty"`
        Owner     string   `json:"owner,omitempty"`
}

// ConfirmedInCurrency sums the confirmed money of the meter in one currency.
// Estimates never enter the hard limit decision.
func ConfirmedInCurrency(confirmed map[string]float64, currency string) float64 {
        return confirmed[currency]
}

// HardLimitReached compares the confirmed spend of the budget currency with
// the declared hard limit. Only confirmed accruals count: an estimate never
// trips the limit.
func HardLimitReached(confirmed map[string]float64, budget *BudgetVersion) bool {
        if budget == nil || budget.HardLimit == nil {
                return false
        }
        return ConfirmedInCurrency(confirmed, budget.Currency) >= *budget.HardLimit
}

// DeriveAdmission computes the admission state from facts: the entitlement
// state, the meter totals and the current budget. Paused, ended and draft
// entitlements block new admissions; a reached hard limit applies the
// declared policy; safety, handoff, reconciliation and accepted
// commitments always continue — the list is part of the answer, never a
// silent cut.
func DeriveAdmission(entitlementState string, confirmed map[string]float64, budget *BudgetVersion) AdmissionState {
        switch entitlementState {
        case StateDraft:
                return AdmissionState{
                        Status: AdmissionBlocked,
                        Reason: "Коммерческий доступ не активирован (draft): новые admission и публикации блокируются.",
                        Policy: "entitlement_state",
                        Continues: ContinuesList, Owner: AdmissionOwner,
                }
        case StatePaused:
                return AdmissionState{
                        Status: AdmissionBlocked,
                        Reason: "Entitlement приостановлен (paused): новые admission и публикации блокируются политикой; существующие данные, аудит, квитанции и recovery сохраняются.",
                        Policy: "entitlement_state",
                        Continues: ContinuesList, Owner: AdmissionOwner,
                }
        case StateEnded:
                return AdmissionState{
                        Status: AdmissionBlocked,
                        Reason: "Entitlement завершён (ended): новые admission и публикации блокируются; данные, аудит, квитанции и обязательные recovery-процессы сохраняются на установленный срок (удаление — только через workflow Story 1.10).",
                        Policy: "entitlement_state",
                        Continues: ContinuesList, Owner: AdmissionOwner,
                }
        }
        if HardLimitReached(confirmed, budget) {
                switch budget.LimitPolicy {
                case PolicyBlockNew:
                        return AdmissionState{
                                Status: AdmissionBlocked,
                                Reason: "Подтверждённый расход достиг hard limit: новые необязательные операции остановлены; принятые обязательства, safety, handoff и reconciliation продолжаются.",
                                Policy: budget.LimitPolicy,
                                Continues: ContinuesList, Owner: AdmissionOwner,
                        }
                case PolicyDegraded:
                        return AdmissionState{
                                Status: AdmissionDegraded,
                                Reason: "Подтверждённый расход достиг hard limit: действует заранее объявленная политика degraded outcome; safety, handoff, reconciliation и принятые обязательства продолжаются.",
                                Policy: budget.LimitPolicy,
                                Continues: ContinuesList, Owner: AdmissionOwner,
                        }
                default:
                        return AdmissionState{
                                Status: AdmissionOK,
                                Reason: "Подтверждённый расход достиг hard limit; действует уведомительная политика (notify_only): новые операции не блокируются, требуется решение Owner.",
                                Policy: budget.LimitPolicy,
                                Continues: ContinuesList, Owner: AdmissionOwner,
                        }
                }
        }
        return AdmissionState{Status: AdmissionOK}
}

// LimitsCheckResult is the honest outcome of the resume-time limits
// re-check, recorded in the resume event.
func LimitsCheckResult(confirmed map[string]float64, budget *BudgetVersion, now time.Time) string {
        if budget == nil {
                return "no_budget"
        }
        if now.Before(budget.PeriodStart) || now.After(budget.PeriodEnd.Add(24*time.Hour)) {
                return "no_budget"
        }
        if HardLimitReached(confirmed, budget) {
                return "exceeded"
        }
        return "ok"
}
