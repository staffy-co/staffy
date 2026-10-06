package domain

import (
        "errors"
        "time"
)

// Errors of the privacy module. Authorization failures surface as ErrNotFound
// from the application layer (tenant isolation is not disclosed).
var (
        ErrNotFound            = errors.New("privacy resource not found")
        ErrForbidden           = errors.New("principal is not allowed to manage the data perimeter")
        ErrInvalidIdempotency  = errors.New("idempotency key is invalid")
        ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
        ErrUnknownClass        = errors.New("artifact class is not in the system dictionary")
        ErrIncompleteMap       = errors.New("data map entry is incomplete or missing")
)

// RuleError is a named validation rejection with an honest human reason.
type RuleError struct {
        Rule   string
        Reason string
}

func (e RuleError) Error() string { return e.Reason }

// Deletion outcomes: the honest dictionary. `deleted` counts only copies the
// map declared and an executor actually removed; `scheduled` names the due
// date; `retained_by_law` carries the basis; `unsupported` carries a managed
// block; `unknown` carries an owner, a deadline and reconciliation.
const (
        OutcomeDeleted      = "deleted"
        OutcomeScheduled    = "scheduled"
        OutcomeRetainedByLaw = "retained_by_law"
        OutcomeUnsupported  = "unsupported"
        OutcomeUnknown      = "unknown"
)

// Map entry completeness: `complete` or `incomplete`. An unknown mandatory
// attribute is stored empty and marked honestly, never guessed.
const (
        MapStatusComplete   = "complete"
        MapStatusIncomplete = "incomplete"
)

// Hold states derived from append-only events.
const (
        HoldNone              = "none"
        HoldActive            = "active"
        HoldRequiresDecision  = "requires_decision"
)

// DataMapEntry is one append-only version of the artifact data map record of
// one class. Unknown mandatory attributes stay empty: the entry is then
// `incomplete` and blocks the class route instead of being filled by guess.
type DataMapEntry struct {
        TenantID         string       `json:"tenant_id"`
        Class            ArtifactClass `json:"class"`
        Version          int64        `json:"version"`
        Purpose          string       `json:"purpose"`
        LegalBasis       string       `json:"legal_basis"`
        Controller       string       `json:"controller"`
        Processor        string       `json:"processor"`
        Recipients       []string     `json:"recipients"`
        SubProcessors    []string     `json:"sub_processors"`
        Locations        []string     `json:"locations"`
        AccessibleRoles  []string     `json:"accessible_roles"`
        KnownCopies      []string     `json:"known_copies"`
        RetentionPolicy  string       `json:"retention_policy_ref"`
        DeletionMethod   string       `json:"deletion_method"`
        ExportMethod     string       `json:"export_method"`
        ActorID          string       `json:"actor_id"`
        CreatedAt        time.Time    `json:"created_at"`
}

// MandatoryAttributes lists the map attributes that must be known for the
// class route to be usable. The order is used for honest missing-attribute
// reporting.
var MandatoryAttributes = []string{
        "purpose", "legal_basis", "controller", "processor",
        "recipients", "sub_processors", "locations", "accessible_roles",
        "known_copies", "retention_policy_ref", "deletion_method", "export_method",
}

// MissingAttributes names the mandatory attributes that are unknown (empty).
func (e DataMapEntry) MissingAttributes() []string {
        missing := []string{}
        values := map[string]any{
                "purpose": e.Purpose, "legal_basis": e.LegalBasis,
                "controller": e.Controller, "processor": e.Processor,
                "recipients": e.Recipients, "sub_processors": e.SubProcessors,
                "locations": e.Locations, "accessible_roles": e.AccessibleRoles,
                "known_copies": e.KnownCopies, "retention_policy_ref": e.RetentionPolicy,
                "deletion_method": e.DeletionMethod, "export_method": e.ExportMethod,
        }
        for _, name := range MandatoryAttributes {
                switch v := values[name].(type) {
                case string:
                        if v == "" {
                                missing = append(missing, name)
                        }
                case []string:
                        if len(v) == 0 {
                                missing = append(missing, name)
                        }
                }
        }
        return missing
}

// Status derives the honest map status of this entry version.
func (e DataMapEntry) Status() string {
        if len(e.MissingAttributes()) == 0 {
                return MapStatusComplete
        }
        return MapStatusIncomplete
}

// RetentionPolicy is one append-only versioned retention policy bound to a
// single artifact class. A blanket policy without a class cannot exist: the
// class is part of the identity of the record.
type RetentionPolicy struct {
        TenantID           string        `json:"tenant_id"`
        Class              ArtifactClass `json:"class"`
        Version            int64         `json:"version"`
        Purpose            string        `json:"purpose"`
        LegalBasis         string        `json:"legal_basis"`
        Source             string        `json:"source"`
        TerminalEvent      string        `json:"terminal_event"`
        ActiveUseDays      int           `json:"active_use_days"`
        ArchiveDays        int           `json:"archive_days"`
        DeletionDays       int           `json:"deletion_days"`
        BackupExpiryDays   int           `json:"backup_expiry_days"`
        LegalHoldException string        `json:"legal_hold_exception"`
        OwnerNote          string        `json:"owner_note"`
        ActorID            string        `json:"actor_id"`
        CreatedAt          time.Time     `json:"created_at"`
}

// RetentionSources are the allowed provenance values of a policy.
var RetentionSources = map[string]bool{
        "tenant_decision":   true,
        "legal_requirement": true,
        "contract":          true,
        "default":           true,
}

// TerminalEvents is the allowed dictionary of term anchor events. In this
// slice the anchor is computed from the last change of the class data; the
// named event stays part of the honest policy contract.
var TerminalEvents = map[string]bool{
        "last_change":        true,
        "source_retired":     true,
        "employee_deleted":   true,
        "invitation_expired": true,
        "membership_revoked": true,
        "connection_revoked": true,
        "preview_expired":    true,
        "tenant_closure":     true,
}

// Day is the unit of policy terms.
const retentionDay = 24 * time.Hour

// DeletionDueAt computes the moment the class data becomes deletable: the
// term anchor plus active use plus archive terms.
func (p RetentionPolicy) DeletionDueAt(anchor time.Time) time.Time {
        return anchor.Add(time.Duration(p.ActiveUseDays+p.ArchiveDays) * retentionDay)
}

// MandatoryDueAt computes the latest allowed retention moment (the overdue
// boundary shown in the map due positions).
func (p RetentionPolicy) MandatoryDueAt(anchor time.Time) time.Time {
        return anchor.Add(time.Duration(p.DeletionDays) * retentionDay)
}

// VoicePreviewInvariantDays is the security invariant of the 24-hour preview
// purge (migration 00008): a retention policy of the preview media class may
// not promise a longer life than the invariant already guarantees.
const VoicePreviewInvariantDays = 1

// HonorsVoicePreviewInvariant reports whether the policy terms stay within
// the 24-hour preview purge invariant.
func (p RetentionPolicy) HonorsVoicePreviewInvariant() bool {
        if p.Class != ClassVoicePreviewMedia {
                return true
        }
        return p.ActiveUseDays+p.ArchiveDays <= VoicePreviewInvariantDays
}

// LegalHoldEvent is one append-only hold event: activation or release of a
// hold over a set of classes with a basis, an approver and a review deadline.
type LegalHoldEvent struct {
        TenantID       string         `json:"tenant_id"`
        EventID        string         `json:"event_id"`
        Event          string         `json:"event"` // activated / released
        Classes        []ArtifactClass `json:"classes"`
        Basis          string         `json:"basis"`
        Approver       string         `json:"approver"`
        ReviewDeadline time.Time      `json:"review_deadline"`
        ActorID        string         `json:"actor_id"`
        CreatedAt      time.Time      `json:"created_at"`
}

// HoldState is the derived hold state of one class at a point in time.
type HoldState struct {
        Status         string    `json:"status"` // none / active / requires_decision
        Basis          string    `json:"basis"`
        Approver       string    `json:"approver"`
        ReviewDeadline time.Time `json:"review_deadline"`
        Since          time.Time `json:"since"`
}

// DeriveHolds computes the hold state of every class from the append-only
// events: the latest event naming the class decides (created_at with the
// event id as the tie-breaker, so a release recorded in the same instant as
// the activation still wins). An active hold whose review deadline has passed
// is reported as requiring a decision, never as silently active.
func DeriveHolds(events []LegalHoldEvent, now time.Time) map[ArtifactClass]HoldState {
        latest := map[ArtifactClass]LegalHoldEvent{}
        for _, e := range events {
                for _, class := range e.Classes {
                        current, seen := latest[class]
                        if !seen || e.CreatedAt.After(current.CreatedAt) || (e.CreatedAt.Equal(current.CreatedAt) && e.EventID > current.EventID) {
                                latest[class] = e
                        }
                }
        }
        states := map[ArtifactClass]HoldState{}
        for class, e := range latest {
                state := HoldState{Status: HoldNone, Since: e.CreatedAt}
                if e.Event == "activated" {
                        state.Status = HoldActive
                        state.Basis = e.Basis
                        state.Approver = e.Approver
                        state.ReviewDeadline = e.ReviewDeadline
                        if !e.ReviewDeadline.After(now) {
                                state.Status = HoldRequiresDecision
                        }
                }
                states[class] = state
        }
        return states
}

// DeletionOutcome is the per-class result of one deletion request. It never
// contains deleted content: counters, dates and honest reasons only.
type DeletionOutcome struct {
        Class                  ArtifactClass `json:"class"`
        Outcome                string        `json:"outcome"`
        AffectedCount          int64         `json:"affected_count"`
        DueAt                  *time.Time    `json:"due_at,omitempty"`
        Note                   string        `json:"note"`
        Block                  *ManagedBlock `json:"block,omitempty"`
        ReconciliationOwner    string        `json:"reconciliation_owner,omitempty"`
        ReconciliationDeadline *time.Time    `json:"reconciliation_deadline,omitempty"`
}

// ReceiptLine is the compact journal projection of one outcome row.
type ReceiptLine struct {
        Class         ArtifactClass `json:"class"`
        Outcome       string        `json:"outcome"`
        AffectedCount int64         `json:"affected_count"`
}

// Receipt is the Data Lifecycle Receipt of one deletion request execution.
type Receipt struct {
        TenantID       string        `json:"tenant_id"`
        ReceiptID      string        `json:"receipt_id"`
        RequestID      string        `json:"request_id"`
        ReceiptHash    string        `json:"receipt_hash"`
        Authority      string        `json:"authority"`
        PolicyVersions []PolicyPin   `json:"policy_versions"`
        Outcomes       []DeletionOutcome `json:"outcomes"`
        CompletedAt    time.Time     `json:"completed_at"`
        ActorID        string        `json:"actor_id"`
}

// PolicyPin anchors one applied retention policy version in the receipt.
type PolicyPin struct {
        Class          ArtifactClass `json:"class"`
        Version        int64         `json:"version"`
        TerminalEvent  string        `json:"terminal_event"`
}
