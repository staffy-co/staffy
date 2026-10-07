package domain

import (
        "crypto/sha256"
        "encoding/hex"
        "encoding/json"
        "errors"
        "fmt"
        "sort"
        "time"

        "github.com/artemiosu/staffy/internal/platform/id"
)

// Errors of the operating profile surface. Authorization failures of members
// surface as ErrProfileForbidden (an honest refusal); unknown employees or
// tenants as ErrNotFound; approval without step-up as ErrProfileStepUpRequired
// (the membership precedent: no provider is wired, so the refusal is honest);
// optimistic-concurrency losses as ProfileConflictError, which carries the
// current draft state for an honest comparison.
var (
        ErrProfileForbidden      = errors.New("principal is not allowed on the operating profile surface")
        ErrProfileStepUpRequired = errors.New("profile approval requires step-up confirmation")
        ErrProfileCorrupt        = errors.New("stored profile content failed the integrity re-check")
)

// Problem kinds of the profile validation. The three kinds map to the
// three honest reactions of the spec: an invalid present value rejects the
// draft save (defaults are never substituted); a missing mandatory piece is
// stored honestly and blocks only the approval (the readiness derivation
// leads to the concrete field); an advisory names a route-level fail-closed
// state (an unproven fallback blocks the route, not the profile).
const (
        ProfileProblemInvalid  = "invalid"
        ProfileProblemMissing  = "missing"
        ProfileProblemAdvisory = "advisory"
)

// ProfileRuleError is a named validation rejection that leads to one concrete
// section and field — the readiness list and the honest form errors share it.
// Problems carries the full list when the rejection flattens many findings:
// every item still leads to its own concrete section and field.
type ProfileRuleError struct {
        Section string
        Field   string
        Reason  string
        Kind    string
        Problems []ProfileRuleError
}

func (e ProfileRuleError) Error() string { return e.Section + "." + e.Field + ": " + e.Reason }
func (e ProfileRuleError) Unwrap() error { return ErrInvalid }

// ProfileConflictError is the honest optimistic-concurrency loss: the expected
// and the current draft revision travel with the current content so the caller
// can compare instead of blindly retrying.
type ProfileConflictError struct {
        Expected       int64
        Current        int64
        CurrentContent *ProfileContent
}

func (e ProfileConflictError) Error() string {
        return fmt.Sprintf("operating profile draft revision conflict: expected %d, current %d", e.Expected, e.Current)
}
func (e ProfileConflictError) Unwrap() error { return ErrConflict }

// ---------------------------------------------------------------------------
// Fixed dictionaries (PRD-aligned; also mirrored in the SQL CHECKs)
// ---------------------------------------------------------------------------

// Conversation Modes are fixed by the PRD dictionary.
var conversationModeDictionary = []string{"sales", "service_status", "safety", "claim", "cancellation", "privacy_no_contact", "human_requested"}

// Fallback kinds of human coverage. voicemail, queue and technical_transfer
// are NOT a confirmed handoff without capability evidence.
var fallbackKindDictionary = []string{"human_callback", "voicemail", "queue", "technical_transfer"}

// Processing purposes of privacy assignments: every allowed route must
// declare each of the five separately.
var processingPurposeDictionary = []string{"recording", "stt_model_processing", "media_processing", "transfer", "marketing_contact"}

// Operational levels. RTO/RPO are goals, never proofs.
var opsLevelDictionary = []string{"best_effort", "commercial"}

// Fail-closed fallbacks of gate declarations.
var gateFallbackDictionary = []string{"block_route", "block_stage", "degraded"}

// Route directions.
const (
        DirectionInbound  = "inbound"
        DirectionOutbound = "outbound"
)

// OQ-11 states of a privacy assignment in use.
const (
        OQ11Open = "open"
        OQ11Pass = "pass"
)

// Route states derived for future admission consumers (Story 1-14, Epic 2).
const (
        RouteStateBlocked = "blocked"
        RouteStateClear   = "clear"
)

// Matrix cell states: a missing cell is an explicit gap, an explicit refusal
// is not_admitted — nothing is hidden and nothing is silently admitted.
const (
        MatrixCellCovered     = "covered"
        MatrixCellGap         = "gap"
        MatrixCellNotAdmitted = "not_admitted"
)

// Threshold states: until the declared evidence owner decides, the gate
// honestly stays pending baseline — the system never invents a number.
const (
        ThresholdStatePendingBaseline = "pending_baseline"
        ThresholdStateDecided         = "decided"
)

// ProfileApplicability states.
const (
        ProfileApplicabilityNone       = "none"               // no approved version yet
        ProfileApplicabilityCurrent    = "current"            // approved and applicable
        ProfileApplicabilityStale      = "stale"              // a newer manifest is sealed
        ProfileApplicabilityUnapproved = "unapproved_changes" // draft edited after approval
)

var conversationModeIndex = indexDictionary(conversationModeDictionary)
var fallbackKindIndex = indexDictionary(fallbackKindDictionary)
var processingPurposeIndex = indexDictionary(processingPurposeDictionary)
var opsLevelIndex = indexDictionary(opsLevelDictionary)
var gateFallbackIndex = indexDictionary(gateFallbackDictionary)

func indexDictionary(values []string) map[string]bool {
        index := make(map[string]bool, len(values))
        for _, value := range values {
                index[value] = true
        }
        return index
}

// ConversationModeDictionary returns the fixed dictionary in order.
func ConversationModeDictionary() []string {
        return append([]string(nil), conversationModeDictionary...)
}

// FallbackKindDictionary returns the fixed dictionary in order.
func FallbackKindDictionary() []string { return append([]string(nil), fallbackKindDictionary...) }

// ProcessingPurposeDictionary returns the fixed dictionary in order.
func ProcessingPurposeDictionary() []string {
        return append([]string(nil), processingPurposeDictionary...)
}

// ---------------------------------------------------------------------------
// Route
// ---------------------------------------------------------------------------

// Route is one working route: a channel from the Brief dictionary plus a
// direction. phone routes are the voice routes.
type Route struct {
        Channel   string `json:"channel"`
        Direction string `json:"direction"`
}

func (r Route) String() string { return r.Channel + ":" + r.Direction }

func (r Route) valid() bool {
        return allowedChannels[r.Channel] && (r.Direction == DirectionInbound || r.Direction == DirectionOutbound)
}

// IsVoice reports whether the route is a voice (phone) route.
func (r Route) IsVoice() bool { return r.Channel == "phone" }

// ---------------------------------------------------------------------------
// Manifest link
// ---------------------------------------------------------------------------

// ManifestLink is the exact reference to the sealed release manifest: the
// identifier, the configuration version and the digest. Every draft write
// re-verifies the link against the stored manifest row; the reference pins
// the exact version, never "the current state".
type ManifestLink struct {
        ManifestID    id.UUID `json:"manifest_id"`
        ConfigVersion int64   `json:"config_version"`
        Digest        string  `json:"digest"`
}

// Equal compares two exact links.
func (l ManifestLink) Equal(other ManifestLink) bool {
        return l.ManifestID == other.ManifestID && l.ConfigVersion == other.ConfigVersion && l.Digest == other.Digest
}

// ---------------------------------------------------------------------------
// Profile content: the sections
// ---------------------------------------------------------------------------

// ProfileContent is the whole editable content of an operating profile. It
// never contains secrets, secret hints, raw PII, transcripts or protected
// media — identifiers, references and decisions only.
type ProfileContent struct {
        WorkScope       WorkScope           `json:"work_scope"`
        Coverage        []HumanCoverage     `json:"human_coverage"`
        Privacy         []PrivacyAssignment `json:"privacy_assignments"`
        OpsLevel        OperationalLevel    `json:"operational_level"`
        Cohort          *PilotCohort        `json:"pilot_cohort,omitempty"`
        Metrics         []MetricDefinition  `json:"measurement_dictionary"`
        Gates           []GateDeclaration   `json:"gate_declarations"`
        Matrix          []CoverageCell      `json:"coverage_matrix"`
        Modes           []string            `json:"conversation_modes"`
        AutonomyClasses []string            `json:"autonomy_classes"`
}

// WorkScope is the working area: routes, prohibitions, languages, the work
// window with an IANA zone, concurrency limits and admission stop conditions.
type WorkScope struct {
        Routes         []Route          `json:"routes"`
        Forbidden      []ForbiddenRoute `json:"forbidden_routes"`
        Languages      []string         `json:"languages"`
        Window         WorkWindow       `json:"window"`
        Limits         WorkLimits       `json:"limits"`
        StopConditions []string         `json:"admission_stop_conditions"`
}

// ForbiddenRoute is an explicit prohibition with a reason.
type ForbiddenRoute struct {
        Route  Route  `json:"route"`
        Reason string `json:"reason"`
}

// WorkWindow is the working window: days, start, end and an IANA time zone
// (validated by loading the zone).
type WorkWindow struct {
        Days     []string `json:"days"`
        Start    string   `json:"start"`
        End      string   `json:"end"`
        TimeZone string   `json:"time_zone"`
}

// WorkLimits are the concurrency limits: per tenant and per route. Calls
// apply to voice routes only.
type WorkLimits struct {
        MaxConcurrentConversations int64        `json:"max_concurrent_conversations_tenant"`
        MaxConcurrentCalls         int64        `json:"max_concurrent_calls_tenant"`
        PerRoute                   []RouteLimit `json:"per_route"`
}

// RouteLimit is the per-route concurrency limit.
type RouteLimit struct {
        Route                      Route `json:"route"`
        MaxConcurrentConversations int64 `json:"max_concurrent_conversations"`
        MaxConcurrentCalls         int64 `json:"max_concurrent_calls"`
}

// HumanCoverage is the mandatory human coverage of one allowed route.
// voicemail/queue/technical_transfer are not a confirmed handoff without
// capability evidence.
type HumanCoverage struct {
        Route              Route    `json:"route"`
        ResponseOwnerID    id.UUID  `json:"response_owner_id"`
        BackupOwnerID      id.UUID  `json:"backup_owner_id"`
        Days               []string `json:"days"`
        Start              string   `json:"start"`
        End                string   `json:"end"`
        AckDeadlineMinutes int64    `json:"ack_deadline_minutes"`
        Fallback           string   `json:"fallback"`
        FallbackEvidence   string   `json:"fallback_evidence,omitempty"`
}

// PrivacyAssignment is one (route × purpose) processing assignment with its
// own parameters. A single shared consent flag is structurally impossible:
// the model stores assignments separately.
type PrivacyAssignment struct {
        Route        Route  `json:"route"`
        Purpose      string `json:"purpose"`
        InUse        bool   `json:"in_use"`
        LegalBasis   string `json:"legal_basis,omitempty"`
        ConsentRef   string `json:"consent_ref,omitempty"`
        Disclosure   string `json:"disclosure,omitempty"`
        Retention    string `json:"retention,omitempty"`
        Deletion     string `json:"deletion,omitempty"`
        Withdrawal   string `json:"withdrawal_behavior,omitempty"`
        OQ11State    string `json:"oq11_state"`
        OQ11Evidence string `json:"oq11_evidence,omitempty"`
}

// OperationalLevel is the mandatory operational responsibility level. RTO/RPO
// are goals — the display always carries the honest "not proven by a drill"
// note because no drill surface exists in the system.
type OperationalLevel struct {
        Level          string    `json:"level"`
        RTOGoal        string    `json:"rto_goal,omitempty"`
        RPOGoal        string    `json:"rpo_goal,omitempty"`
        ResponsibleIDs []id.UUID `json:"responsible_ids,omitempty"`
        ServiceImpact  string    `json:"service_impact,omitempty"`
}

// PilotCohort is the optional (until the pilot) cohort section. Changing the
// sample composition requires a new profile version with an audit
// explanation — controlled baseline stays structurally separate from the
// later threshold decisions.
type PilotCohort struct {
        Eligibility     []string `json:"eligibility"`
        Exclusions      []string `json:"exclusions,omitempty"`
        Denominator     string   `json:"intent_to_treat_denominator"`
        Assignment      string   `json:"assignment"`
        AttritionPolicy string   `json:"attrition_policy"`
}

// MetricDefinition is one metric of the Measurement Dictionary. Voice and
// text are measured separately through the mandatory modality slice; a
// route-blocking failure cannot be hidden behind an average — the visibility
// field is mandatory.
type MetricDefinition struct {
        ID                     string   `json:"id"`
        Name                   string   `json:"name"`
        BusinessMeaning        string   `json:"business_meaning"`
        Numerator              string   `json:"numerator"`
        Denominator            string   `json:"denominator"`
        SourceOfTruth          string   `json:"source_of_truth"`
        TerminalTiming         string   `json:"terminal_timing"`
        Slices                 []string `json:"slices"`
        OwnerID                id.UUID  `json:"owner_id"`
        DisputePolicy          string   `json:"dispute_resolution"`
        RouteFailureVisibility string   `json:"route_failure_visibility"`
}

// MandatoryMetricSlices lists the slices every metric must include.
var mandatoryMetricSlices = []string{"journey", "modality", "conversation_mode", "release_version", "autonomy_class"}

// MandatoryMetricSlices returns the mandatory slice dictionary in order.
func MandatoryMetricSlices() []string { return append([]string(nil), mandatoryMetricSlices...) }

// GateDeclaration is one declared gate. The numeric threshold is never set
// by the system: until the evidence owner decides, the gate honestly stays
// pending baseline.
type GateDeclaration struct {
        GateID               string  `json:"gate_id"`
        Title                string  `json:"title,omitempty"`
        EvidenceOwnerID      id.UUID `json:"evidence_owner_id"`
        DecisionDeadline     string  `json:"decision_deadline"`
        FailClosedFallback   string  `json:"fail_closed_fallback"`
        AllowRoleCoincidence bool    `json:"allow_role_coincidence"`
}

// CoverageCell is one declared cell of the coverage matrix: either admitted
// work-in-progress (eval link, gate references, human fallback) or an
// explicit not admitted with a reason. Wildcards do not exist.
type CoverageCell struct {
        Journey           string   `json:"journey"`
        Route             Route    `json:"route"`
        ConversationMode  string   `json:"conversation_mode"`
        AutonomyClass     string   `json:"autonomy_class"`
        NotAdmitted       bool     `json:"not_admitted"`
        NotAdmittedReason string   `json:"not_admitted_reason,omitempty"`
        EvalRef           string   `json:"eval_ref,omitempty"`
        Gates             []string `json:"gate_refs,omitempty"`
        HumanFallback     string   `json:"human_fallback,omitempty"`
}

// ---------------------------------------------------------------------------
// Draft, approved version and threshold records
// ---------------------------------------------------------------------------

// ProfileDraft is the only mutable record of the profile surface: a CAS
// revision, the exact manifest link and the editable content. Approved
// versions and threshold decisions are append-only.
type ProfileDraft struct {
        TenantID         id.UUID        `json:"tenant_id"`
        EmployeeID       id.UUID        `json:"employee_id"`
        Revision         int64          `json:"revision"`
        ApprovedRevision *int64         `json:"approved_revision,omitempty"`
        Manifest         ManifestLink   `json:"manifest"`
        Content          ProfileContent `json:"content"`
        ActorID          id.UUID        `json:"actor_id"`
        UpdatedAt        time.Time      `json:"updated_at"`
}

// ProfileVersion is the immutable, append-only approved profile. The digest
// pins the canonical content; approver roles are recorded at approval time.
type ProfileVersion struct {
        TenantID       id.UUID        `json:"tenant_id"`
        EmployeeID     id.UUID        `json:"employee_id"`
        Version        int64          `json:"version"`
        Manifest       ManifestLink   `json:"manifest"`
        Content        ProfileContent `json:"content"`
        Digest         string         `json:"digest"`
        ApproverID     id.UUID        `json:"approver_id"`
        ApproverRoles  []string       `json:"approver_roles"`
        Description    string         `json:"description"`
        EvidenceRefs   []string       `json:"evidence_refs"`
        StepUpVerified bool           `json:"step_up_verified"`
        ApprovedAt     time.Time      `json:"approved_at"`
}

// ThresholdLink points at the previous threshold version a decision
// supersedes — the history is never rewritten.
type ThresholdLink struct {
        ThresholdVersion int64     `json:"threshold_version"`
        DecidedAt        time.Time `json:"decided_at"`
}

// GateThresholdVersion is one append-only threshold decision. The system does
// not claim a machine verification against a controlled baseline — the value
// is a human assertion with evidence references.
type GateThresholdVersion struct {
        TenantID         id.UUID        `json:"tenant_id"`
        EmployeeID       id.UUID        `json:"employee_id"`
        GateID           string         `json:"gate_id"`
        ThresholdVersion int64          `json:"threshold_version"`
        Value            string         `json:"value"`
        ActorID          id.UUID        `json:"actor_id"`
        Decision         string         `json:"decision"`
        EvidenceRefs     []string       `json:"evidence_refs"`
        Supersedes       *ThresholdLink `json:"supersedes,omitempty"`
        DecidedAt        time.Time      `json:"decided_at"`
}

// ---------------------------------------------------------------------------
// Canonical serialization and digest
// ---------------------------------------------------------------------------

// canonicalizeProfile returns a copy with every repeated slice in the fixed
// canonical order the digest depends on.
func canonicalizeProfile(content ProfileContent) ProfileContent {
        out := content
        out.WorkScope.Routes = sortedRoutes(content.WorkScope.Routes)
        out.WorkScope.Languages = sortedCopy(content.WorkScope.Languages)
        out.WorkScope.StopConditions = sortedCopy(content.WorkScope.StopConditions)
        out.WorkScope.Window.Days = sortedCopy(content.WorkScope.Window.Days)
        sort.SliceStable(out.WorkScope.Forbidden, func(i, j int) bool {
                return out.WorkScope.Forbidden[i].Route.String() < out.WorkScope.Forbidden[j].Route.String()
        })
        sort.SliceStable(out.WorkScope.Limits.PerRoute, func(i, j int) bool {
                return out.WorkScope.Limits.PerRoute[i].Route.String() < out.WorkScope.Limits.PerRoute[j].Route.String()
        })
        out.Coverage = sortedCoverage(content.Coverage)
        out.Privacy = sortedPrivacy(content.Privacy)
        out.OpsLevel.ResponsibleIDs = sortedIDs(content.OpsLevel.ResponsibleIDs)
        if out.Cohort != nil {
                cohort := *out.Cohort
                cohort.Eligibility = sortedCopy(cohort.Eligibility)
                cohort.Exclusions = sortedCopy(cohort.Exclusions)
                out.Cohort = &cohort
        }
        sort.SliceStable(out.Metrics, func(i, j int) bool { return out.Metrics[i].ID < out.Metrics[j].ID })
        for index := range out.Metrics {
                out.Metrics[index].Slices = sortedCopy(out.Metrics[index].Slices)
        }
        sort.SliceStable(out.Gates, func(i, j int) bool { return out.Gates[i].GateID < out.Gates[j].GateID })
        out.Matrix = sortedMatrix(content.Matrix)
        out.Modes = sortedCopy(content.Modes)
        out.AutonomyClasses = sortedCopy(content.AutonomyClasses)
        return out
}

func sortedCopy(values []string) []string {
        out := append([]string(nil), values...)
        sort.Strings(out)
        return out
}

func sortedIDs(values []id.UUID) []id.UUID {
        out := append([]id.UUID(nil), values...)
        sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
        return out
}

func sortedRoutes(routes []Route) []Route {
        out := append([]Route(nil), routes...)
        sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
        return out
}

func sortedCoverage(rows []HumanCoverage) []HumanCoverage {
        out := append([]HumanCoverage(nil), rows...)
        sort.SliceStable(out, func(i, j int) bool { return out[i].Route.String() < out[j].Route.String() })
        for index := range out {
                out[index].Days = sortedCopy(out[index].Days)
        }
        return out
}

func sortedPrivacy(rows []PrivacyAssignment) []PrivacyAssignment {
        out := append([]PrivacyAssignment(nil), rows...)
        sort.SliceStable(out, func(i, j int) bool {
                if out[i].Route.String() != out[j].Route.String() {
                        return out[i].Route.String() < out[j].Route.String()
                }
                return out[i].Purpose < out[j].Purpose
        })
        return out
}

func sortedMatrix(cells []CoverageCell) []CoverageCell {
        out := append([]CoverageCell(nil), cells...)
        sort.SliceStable(out, func(i, j int) bool {
                a, b := out[i], out[j]
                if a.Journey != b.Journey {
                        return a.Journey < b.Journey
                }
                if a.Route.String() != b.Route.String() {
                        return a.Route.String() < b.Route.String()
                }
                if a.ConversationMode != b.ConversationMode {
                        return a.ConversationMode < b.ConversationMode
                }
                return a.AutonomyClass < b.AutonomyClass
        })
        for index := range out {
                out[index].Gates = sortedCopy(out[index].Gates)
        }
        return out
}

// CanonicalProfileJSON serializes the content deterministically: struct field
// order is fixed by the encoder, repeated slices are pre-sorted by the
// canonicalizer.
func CanonicalProfileJSON(content ProfileContent) ([]byte, error) {
        return json.Marshal(canonicalizeProfile(content))
}

// ProfileDigest computes the sha256 hex digest of the canonical bytes. It is
// computed over the approved content and never recomputed for an on-the-spot
// "migration".
func ProfileDigest(content ProfileContent) (string, error) {
        raw, err := CanonicalProfileJSON(content)
        if err != nil {
                return "", err
        }
        sum := sha256.Sum256(raw)
        return hex.EncodeToString(sum[:]), nil
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// ProfileFacts are the caller-authority facts the references are validated
// against: the member identifiers of the tenant and the journeys of the pack
// pinned by the sealed manifest.
type ProfileFacts struct {
        MemberIDs    map[id.UUID]bool
        PackJourneys []string
}

// HasInvalidProfileProblems reports whether the list contains a present but
// invalid value — the kind that rejects the draft save.
func HasInvalidProfileProblems(problems []ProfileRuleError) bool {
        for _, problem := range problems {
                if problem.Kind == ProfileProblemInvalid {
                        return true
                }
        }
        return false
}

// HasBlockingProfileProblems reports whether the list contains an invalid or
// missing problem — both kinds block the approval (advisory route-level
// states do not: blocked routes are honestly visible and never prevent the
// approval; admission is Story 1-14).
func HasBlockingProfileProblems(problems []ProfileRuleError) bool {
        for _, problem := range problems {
                if problem.Kind == ProfileProblemInvalid || problem.Kind == ProfileProblemMissing {
                        return true
                }
        }
        return false
}

// ValidateProfileContent validates every section against the fixed
// dictionaries and the facts. It returns the full honest list — the same
// list doubles as the approval readiness derivation (each item leads to one
// concrete section and field). Every problem carries its kind: invalid
// (present but wrong), missing (mandatory piece absent) or advisory (a
// route-level fail-closed state).
func ValidateProfileContent(content ProfileContent, facts ProfileFacts) []ProfileRuleError {
        problems := []ProfileRuleError{}
        section := func(name, kind string) func(field, reason string) {
                return func(field, reason string) {
                        problems = append(problems, ProfileRuleError{Section: name, Field: field, Reason: reason, Kind: kind})
                }
        }
        invalid := func(name string) func(field, reason string) { return section(name, ProfileProblemInvalid) }
        missing := func(name string) func(field, reason string) { return section(name, ProfileProblemMissing) }
        advisory := func(name string) func(field, reason string) { return section(name, ProfileProblemAdvisory) }
        add := invalid("work_scope")
        absent := missing("work_scope")
        routes := map[string]Route{}
        hasVoiceRoute := false
        if len(content.WorkScope.Routes) == 0 {
                absent("routes", "Укажите хотя бы один разрешённый маршрут (канал + направление).")
        }
        for _, route := range content.WorkScope.Routes {
                if !route.valid() {
                        add("routes", "Маршрут "+route.String()+" вне словаря: канал — из словаря Brief, направление — inbound или outbound.")
                        continue
                }
                if _, seen := routes[route.String()]; seen {
                        add("routes", "Маршрут "+route.String()+" объявлен дважды: дубликаты запрещены.")
                        continue
                }
                routes[route.String()] = route
                if route.IsVoice() {
                        hasVoiceRoute = true
                }
        }
        if len(content.WorkScope.Forbidden) > 24 {
                add("forbidden_routes", "Слишком много записей о запрещённых маршрутах (не более 24).")
        }
        forbidden := map[string]bool{}
        for _, row := range content.WorkScope.Forbidden {
                if !row.Route.valid() {
                        add("forbidden_routes", "Запрещённый маршрут "+row.Route.String()+" вне словаря каналов и направлений.")
                        continue
                }
                if forbidden[row.Route.String()] {
                        add("forbidden_routes", "Запрещённый маршрут "+row.Route.String()+" объявлен дважды.")
                        continue
                }
                forbidden[row.Route.String()] = true
                if routes[row.Route.String()] != (Route{}) {
                        add("forbidden_routes", "Маршрут "+row.Route.String()+" одновременно разрешён и запрещён — это противоречие.")
                }
                if !length(row.Reason, 1, 300) {
                        if row.Reason == "" {
                                absent("forbidden_routes", "Причина запрета маршрута "+row.Route.String()+" не указана (1–300 символов).")
                        } else {
                                add("forbidden_routes", "Причина запрета маршрута "+row.Route.String()+": не более 300 символов.")
                        }
                }
        }
        if len(content.WorkScope.Languages) == 0 {
                absent("languages", "Языки не заданы: непустой список до 12 значений.")
        } else if len(content.WorkScope.Languages) > 12 {
                add("languages", "Слишком много языков (не более 12).")
        } else {
                seen := map[string]bool{}
                for _, language := range content.WorkScope.Languages {
                        if !length(language, 1, 160) || seen[language] {
                                add("languages", "Язык «"+language+"» задан неверно или повторяется.")
                                break
                        }
                        seen[language] = true
                }
        }
        if workWindowEmpty(content.WorkScope.Window) {
                absent("window", "Окно работы не задано: дни недели, начало и конец в формате ЧЧ:ММ, таймзона IANA.")
        } else if !workingWindowValid(content.WorkScope.Window) {
                add("window", "Окно работы: дни недели без повторов, начало и конец в формате ЧЧ:ММ, конец позже начала, таймзона — IANA.")
        }
        if content.WorkScope.Limits.MaxConcurrentConversations < 1 {
                if content.WorkScope.Limits.MaxConcurrentConversations == 0 {
                        absent("limits", "Лимит одновременных Conversations по Tenant не указан: целое число ≥ 1.")
                } else {
                        add("limits", "Лимит одновременных Conversations по Tenant: целое число ≥ 1.")
                }
        }
        if hasVoiceRoute && content.WorkScope.Limits.MaxConcurrentCalls < 1 {
                if content.WorkScope.Limits.MaxConcurrentCalls == 0 {
                        absent("limits", "Лимит одновременных calls по Tenant не указан (голосовые маршруты объявлены): целое число ≥ 1.")
                } else {
                        add("limits", "Лимит одновременных calls по Tenant: целое число ≥ 1 (голосовые маршруты объявлены).")
                }
        }
        perRouteLimits := map[string]RouteLimit{}
        for _, limit := range content.WorkScope.Limits.PerRoute {
                if routes[limit.Route.String()] == (Route{}) {
                        add("limits", "Лимит маршрута "+limit.Route.String()+": маршрут не объявлен в разрешённых.")
                        continue
                }
                if _, seen := perRouteLimits[limit.Route.String()]; seen {
                        add("limits", "Лимит маршрута "+limit.Route.String()+" объявлен дважды.")
                        continue
                }
                if limit.MaxConcurrentConversations < 1 {
                        if limit.MaxConcurrentConversations == 0 {
                                absent("limits", "Лимит Conversations маршрута "+limit.Route.String()+" не указан: целое число ≥ 1.")
                        } else {
                                add("limits", "Лимит Conversations маршрута "+limit.Route.String()+": целое число ≥ 1.")
                        }
                }
                if limit.Route.IsVoice() && limit.MaxConcurrentCalls < 1 {
                        if limit.MaxConcurrentCalls == 0 {
                                absent("limits", "Лимит calls маршрута "+limit.Route.String()+" не указан (голосовой маршрут): целое число ≥ 1.")
                        } else {
                                add("limits", "Лимит calls маршрута "+limit.Route.String()+": целое число ≥ 1 (голосовой маршрут).")
                        }
                }
                if !limit.Route.IsVoice() && limit.MaxConcurrentCalls != 0 {
                        add("limits", "Лимит calls маршрута "+limit.Route.String()+": поле применимо только к голосовым маршрутам.")
                }
                perRouteLimits[limit.Route.String()] = limit
        }
        for key := range routes {
                if _, ok := perRouteLimits[key]; !ok {
                        absent("limits", "Лимит маршрута "+key+" отсутствует: каждый разрешённый маршрут требует лимита Conversations.")
                }
        }
        if len(content.WorkScope.StopConditions) == 0 {
                absent("admission_stop_conditions", "Условия остановки admission не заданы: непустой список до 32 записей.")
        } else if len(content.WorkScope.StopConditions) > 32 {
                add("admission_stop_conditions", "Слишком много условий остановки admission (не более 32).")
        } else {
                for _, condition := range content.WorkScope.StopConditions {
                        if !length(condition, 1, 300) {
                                add("admission_stop_conditions", "Каждое условие остановки admission: от 1 до 300 символов.")
                                break
                        }
                }
        }

        // Human coverage: one entry per allowed route.
        add = invalid("human_coverage")
        absent = missing("human_coverage")
        advise := advisory("human_coverage")
        coverage := map[string]HumanCoverage{}
        for _, row := range content.Coverage {
                if routes[row.Route.String()] == (Route{}) {
                        add(row.Route.String(), "Покрытие объявлено для маршрута, отсутствующего в разрешённых.")
                        continue
                }
                if _, seen := coverage[row.Route.String()]; seen {
                        add(row.Route.String(), "Покрытие маршрута объявлено дважды.")
                        continue
                }
                coverage[row.Route.String()] = row
        }
        for key := range routes {
                row, ok := coverage[key]
                if !ok {
                        absent(key, "Обязательное человеческое покрытие маршрута отсутствует.")
                        continue
                }
                if !facts.MemberIDs[row.ResponseOwnerID] {
                        add(key, "Response Owner не является участником Tenant.")
                }
                if !facts.MemberIDs[row.BackupOwnerID] {
                        add(key, "Backup Owner не является участником Tenant.")
                }
                if len(row.Days) == 0 && row.Start == "" && row.End == "" {
                        absent(key, "Часы доступности не заданы: дни без повторов, начало и конец в формате ЧЧ:ММ, конец позже начала.")
                } else if !workingDaysValid(row.Days, row.Start, row.End) {
                        add(key, "Часы доступности: дни без повторов, начало и конец в формате ЧЧ:ММ, конец позже начала.")
                }
                if row.AckDeadlineMinutes < 1 {
                        if row.AckDeadlineMinutes == 0 {
                                absent(key, "Acknowledgement deadline не указан: положительная длительность в минутах (≥ 1).")
                        } else {
                                add(key, "Acknowledgement deadline: положительная длительность в минутах (≥ 1).")
                        }
                }
                if !fallbackKindIndex[row.Fallback] {
                        add(key, "Допустимый fallback: "+joinDictionary(fallbackKindDictionary)+".")
                        continue
                }
                if row.Fallback != "human_callback" && !length(row.FallbackEvidence, 1, 300) {
                        advise(key, "Fallback «"+row.Fallback+"» не является подтверждённым handoff без ссылки на capability evidence: маршрут честно блокируется до появления ссылки (маршрут, а не профиль).")
                }
        }

        // Privacy assignments: every route × every purpose, separate parameters.
        add = invalid("privacy_assignments")
        absent = missing("privacy_assignments")
        assignments := map[string]PrivacyAssignment{}
        for _, row := range content.Privacy {
                if routes[row.Route.String()] == (Route{}) {
                        add(row.Route.String()+" · "+row.Purpose, "Назначение объявлено для маршрута, отсутствующего в разрешённых.")
                        continue
                }
                if !processingPurposeIndex[row.Purpose] {
                        add(row.Route.String(), "Цель обработки вне словаря: "+joinDictionary(processingPurposeDictionary)+".")
                        continue
                }
                key := row.Route.String() + " · " + row.Purpose
                if _, seen := assignments[key]; seen {
                        add(key, "Назначение объявлено дважды.")
                        continue
                }
                assignments[key] = row
        }
        for key := range routes {
                for _, purpose := range processingPurposeDictionary {
                        row, ok := assignments[key+" · "+purpose]
                        if !ok {
                                absent(key+" · "+purpose, "Назначение обязательно для каждой цели словаря: явное «не используется» или параметры.")
                                continue
                        }
                        if !row.InUse {
                                continue
                        }
                        for field, value := range map[string]string{
                                "legal_basis": row.LegalBasis, "consent_ref": row.ConsentRef, "disclosure": row.Disclosure,
                                "retention": row.Retention, "deletion": row.Deletion, "withdrawal_behavior": row.Withdrawal,
                        } {
                                if value == "" {
                                        absent(key, "Назначение в использовании: поле "+field+" не указано (1–300 символов) — общий флаг согласия не допускается.")
                                } else if !length(value, 1, 300) {
                                        add(key, "Назначение в использовании: поле "+field+" — не более 300 символов.")
                                }
                        }
                        if row.OQ11State != OQ11Open && row.OQ11State != OQ11Pass {
                                add(key, "Состояние OQ-11: open или pass.")
                        }
                        if row.OQ11State == OQ11Pass && !length(row.OQ11Evidence, 1, 300) {
                                add(key, "OQ-11 pass требует ссылку на evidence: заявленный pass без ссылки — ложное утверждение, он отвергается.")
                        }
                }
        }

        // Operational level: mandatory; commercial requires goals and owners.
        add = invalid("operational_level")
        absent = missing("operational_level")
        if content.OpsLevel.Level == "" {
                absent("level", "Уровень операционной ответственности не выбран: "+joinDictionary(opsLevelDictionary)+".")
        } else if !opsLevelIndex[content.OpsLevel.Level] {
                add("level", "Уровень операционной ответственности: "+joinDictionary(opsLevelDictionary)+".")
        } else if content.OpsLevel.Level == "commercial" {
                for goal, value := range map[string]string{"rto_goal": content.OpsLevel.RTOGoal, "rpo_goal": content.OpsLevel.RPOGoal} {
                        if value == "" {
                                absent(goal, "Коммерческий уровень требует цели (1–300 символов; цели — не доказательства, «не доказаны до drill»).")
                        } else if !length(value, 1, 300) {
                                add(goal, "Цель "+goal+": не более 300 символов.")
                        }
                }
                if len(content.OpsLevel.ResponsibleIDs) == 0 {
                        absent("responsible_ids", "Коммерческий уровень требует ответственных участников.")
                }
                for _, responsible := range content.OpsLevel.ResponsibleIDs {
                        if !facts.MemberIDs[responsible] {
                                add("responsible_ids", "Ответственный не является участником Tenant.")
                                break
                        }
                }
                if content.OpsLevel.ServiceImpact == "" {
                        absent("service_impact", "Коммерческий уровень требует влияния на условия обслуживания.")
                } else if !length(content.OpsLevel.ServiceImpact, 1, 300) {
                        add("service_impact", "Влияние на условия обслуживания: не более 300 символов.")
                }
        }

        // Cohort: optional until the pilot, but validated when filled.
        add = invalid("pilot_cohort")
        absent = missing("pilot_cohort")
        if content.Cohort != nil {
                cohort := content.Cohort
                if len(cohort.Eligibility) == 0 {
                        absent("eligibility", "Когорта заполнена: укажите критерии включения (или уберите раздел до пилота).")
                }
                if cohort.Denominator == "" {
                        absent("intent_to_treat_denominator", "Когорта заполнена: denominator обязателен.")
                } else if !length(cohort.Denominator, 1, 300) {
                        add("intent_to_treat_denominator", "Denominator: не более 300 символов.")
                }
                if cohort.Assignment == "" {
                        absent("assignment", "Когорта заполнена: правила assignment/matching сопоставимой группы обязательны.")
                } else if !length(cohort.Assignment, 1, 300) {
                        add("assignment", "Правила assignment: не более 300 символов.")
                }
                if cohort.AttritionPolicy == "" {
                        absent("attrition_policy", "Когорта заполнена: обработка отказов и потерь обязательна.")
                } else if !length(cohort.AttritionPolicy, 1, 300) {
                        add("attrition_policy", "Обработка отказов и потерь: не более 300 символов.")
                }
        }

        // Measurement dictionary: at least one metric with all fields.
        add = invalid("measurement_dictionary")
        absent = missing("measurement_dictionary")
        if len(content.Metrics) == 0 {
                absent("metrics", "Measurement Dictionary обязателен: хотя бы одна метрика.")
        }
        seenMetric := map[string]bool{}
        for _, metric := range content.Metrics {
                if !length(metric.ID, 1, 64) {
                        add("metrics", "Идентификатор метрики: 1–64 символа.")
                } else if seenMetric[metric.ID] {
                        add("metrics", "Метрика «"+metric.ID+"» объявлена дважды.")
                }
                seenMetric[metric.ID] = true
                for field, value := range map[string]string{
                        "name": metric.Name, "business_meaning": metric.BusinessMeaning, "numerator": metric.Numerator,
                        "denominator": metric.Denominator, "source_of_truth": metric.SourceOfTruth, "terminal_timing": metric.TerminalTiming,
                        "dispute_resolution": metric.DisputePolicy,
                } {
                        if value == "" {
                                absent(metric.ID+"."+field, "Поле обязательно: 1–300 символов.")
                        } else if !length(value, 1, 300) {
                                add(metric.ID+"."+field, "Поле "+field+": не более 300 символов.")
                        }
                }
                if !facts.MemberIDs[metric.OwnerID] {
                        add(metric.ID+".owner_id", "Владелец метрики не является участником Tenant.")
                }
                slices := map[string]bool{}
                for _, slice := range metric.Slices {
                        slices[slice] = true
                }
                for _, mandatory := range mandatoryMetricSlices {
                        if !slices[mandatory] {
                                absent(metric.ID+".slices", "Срезы обязаны включать "+joinDictionary(mandatoryMetricSlices)+" (голос и текст — раздельно через modality).")
                                break
                        }
                }
                if metric.RouteFailureVisibility == "" {
                        absent(metric.ID+".route_failure_visibility", "Видимость route-blocking failure обязательна: маршрут-блокирующий сбой нельзя скрыть средним показателем.")
                } else if !length(metric.RouteFailureVisibility, 1, 300) {
                        add(metric.ID+".route_failure_visibility", "Видимость route-blocking failure: не более 300 символов.")
                }
        }

        // Gate declarations: at least one, evidence owner must exist.
        add = invalid("gate_declarations")
        absent = missing("gate_declarations")
        if len(content.Gates) == 0 {
                absent("gates", "Gate-декларации обязательны: хотя бы один gate.")
        }
        declaredGates := map[string]bool{}
        for _, gate := range content.Gates {
                if !length(gate.GateID, 1, 64) {
                        add("gates", "Идентификатор gate: 1–64 символа.")
                        continue
                }
                if declaredGates[gate.GateID] {
                        add("gates", "Gate «"+gate.GateID+"» объявлен дважды.")
                }
                declaredGates[gate.GateID] = true
                if !facts.MemberIDs[gate.EvidenceOwnerID] {
                        add(gate.GateID+".evidence_owner_id", "Evidence owner gate не является участником Tenant.")
                }
                if !length(gate.DecisionDeadline, 1, 300) {
                        if gate.DecisionDeadline == "" {
                                absent(gate.GateID+".decision_deadline", "Срок решения gate не указан (1–300 символов).")
                        } else {
                                add(gate.GateID+".decision_deadline", "Срок решения gate: не более 300 символов.")
                        }
                }
                if !gateFallbackIndex[gate.FailClosedFallback] {
                        add(gate.GateID+".fail_closed_fallback", "Fail-closed fallback: "+joinDictionary(gateFallbackDictionary)+".")
                }
        }

        // Declared modes and autonomy classes: non-empty subsets of the dictionaries.
        add = invalid("coverage_matrix")
        absent = missing("coverage_matrix")
        if len(content.Modes) == 0 {
                absent("conversation_modes", "Объявленные Conversation Modes не заданы: непустое подмножество словаря.")
        }
        declaredModes := map[string]bool{}
        for _, mode := range content.Modes {
                if !conversationModeIndex[mode] {
                        add("conversation_modes", "Conversation Mode «"+mode+"» вне словаря: "+joinDictionary(conversationModeDictionary)+".")
                        continue
                }
                if declaredModes[mode] {
                        add("conversation_modes", "Conversation Mode «"+mode+"» объявлен дважды.")
                }
                declaredModes[mode] = true
        }
        if len(content.AutonomyClasses) == 0 {
                absent("autonomy_classes", "Объявленные Autonomy Classes не заданы: непустой список (фиксированных поверхностей классов в системе нет).")
        }
        seenClass := map[string]bool{}
        for _, class := range content.AutonomyClasses {
                if !length(class, 1, 64) {
                        add("autonomy_classes", "Autonomy Class: 1–64 символа.")
                        continue
                }
                if seenClass[class] {
                        add("autonomy_classes", "Autonomy Class «"+class+"» объявлен дважды.")
                }
                seenClass[class] = true
        }
        packJourneys := map[string]bool{}
        for _, journey := range facts.PackJourneys {
                packJourneys[journey] = true
        }
        cellIndex := map[string]CoverageCell{}
        for _, cell := range content.Matrix {
                if routes[cell.Route.String()] == (Route{}) {
                        add(cell.Journey+" · "+cell.Route.String(), "Ячейка ссылается на маршрут, отсутствующий в разрешённых.")
                        continue
                }
                if !packJourneys[cell.Journey] {
                        add(cell.Journey+" · "+cell.Route.String(), "Journey «"+cell.Journey+"» не входит в Pack-ссылку запечатанного манифеста.")
                        continue
                }
                if !declaredModes[cell.ConversationMode] {
                        add(cell.Journey+" · "+cell.Route.String(), "Conversation Mode ячейки не объявлен в профиле.")
                        continue
                }
                if !seenClass[cell.AutonomyClass] {
                        add(cell.Journey+" · "+cell.Route.String(), "Autonomy Class ячейки не объявлен в профиле.")
                        continue
                }
                key := cell.Journey + " · " + cell.Route.String() + " · " + cell.ConversationMode + " · " + cell.AutonomyClass
                if _, seen := cellIndex[key]; seen {
                        add(key, "Ячейка матрицы объявлена дважды.")
                        continue
                }
                cellIndex[key] = cell
                if cell.NotAdmitted {
                        if cell.NotAdmittedReason == "" {
                                absent(key, "Ячейка «не допущено» требует причину.")
                        } else if !length(cell.NotAdmittedReason, 1, 300) {
                                add(key, "Причина «не допущено»: не более 300 символов.")
                        }
                        continue
                }
                if cell.EvalRef == "" {
                        absent(key, "Ячейка требует eval-ссылку (ссылка без исполнения; пусто — только через явное «не допущено»).")
                } else if !length(cell.EvalRef, 1, 300) {
                        add(key, "Eval-ссылка: не более 300 символов.")
                }
                if cell.HumanFallback == "" {
                        absent(key, "Ячейка требует human fallback.")
                } else if !length(cell.HumanFallback, 1, 300) {
                        add(key, "Human fallback: не более 300 символов.")
                }
                for _, gateRef := range cell.Gates {
                        if !declaredGates[gateRef] {
                                add(key, "Ссылка на gate «"+gateRef+"» вне объявленных gate-деклараций.")
                        }
                }
        }
        // Completeness: the cartesian product must be fully decided.
        for _, journey := range sortedCopy(facts.PackJourneys) {
                for _, route := range sortedRoutes(content.WorkScope.Routes) {
                        for _, mode := range sortedCopy(content.Modes) {
                                for _, class := range sortedCopy(content.AutonomyClasses) {
                                        key := journey + " · " + route.String() + " · " + mode + " · " + class
                                        if _, ok := cellIndex[key]; !ok {
                                                absent(key, "Пустая ячейка матрицы — явный gap: заполните ячейку или пометьте «не допущено».")
                                        }
                                }
                        }
                }
        }
        return problems
}

// ValidateThresholdDecision validates the human threshold decision fields.
func ValidateThresholdDecision(value, decision string, evidenceRefs []string) error {
        if !length(value, 1, 300) {
                return ProfileRuleError{Section: "gate_threshold", Field: "value", Reason: "Значение порога: 1–300 символов; система не подставляет значения по умолчанию."}
        }
        if !length(decision, 1, 300) {
                return ProfileRuleError{Section: "gate_threshold", Field: "decision", Reason: "Решение/пояснение: 1–300 символов."}
        }
        if len(evidenceRefs) == 0 {
                return ProfileRuleError{Section: "gate_threshold", Field: "evidence_refs", Reason: "Утверждение порога требует хотя бы одной evidence-ссылки."}
        }
        for _, ref := range evidenceRefs {
                if !length(ref, 1, 300) {
                        return ProfileRuleError{Section: "gate_threshold", Field: "evidence_refs", Reason: "Каждая evidence-ссылка: 1–300 символов."}
                }
        }
        return nil
}

// ValidateProfileDescription checks the approval explanation.
func ValidateProfileDescription(value string) error {
        if !length(value, 1, 300) {
                return ProfileRuleError{Section: "approval", Field: "description", Reason: "Описание-объяснение утверждения: от 1 до 300 символов."}
        }
        return nil
}

// workWindowEmpty reports the fully unfilled window (all four fields empty):
// an honest "missing" instead of a format rejection.
func workWindowEmpty(window WorkWindow) bool {
        return len(window.Days) == 0 && window.Start == "" && window.End == "" && window.TimeZone == ""
}

func workingWindowValid(window WorkWindow) bool {
        if window.TimeZone == "Local" {
                return false
        }
        if _, err := time.LoadLocation(window.TimeZone); err != nil {
                return false
        }
        return workingDaysValid(window.Days, window.Start, window.End)
}

func workingDaysValid(days []string, start, end string) bool {
        if len(days) == 0 || len(days) > 7 {
                return false
        }
        seen := map[string]bool{}
        for _, day := range days {
                if !allowedDays[day] || seen[day] {
                        return false
                }
                seen[day] = true
        }
        return validClock(start) && validClock(end) && start < end
}

func joinDictionary(values []string) string {
        out := ""
        for index, value := range values {
                if index > 0 {
                        out += ", "
                }
                out += value
        }
        return out
}

// ---------------------------------------------------------------------------
// Derivations
// ---------------------------------------------------------------------------

// RouteState is the derived state of one allowed route for the future
// admission consumer (Story 1-14, Epic 2). The profile never admits traffic
// itself: a clear route is "not blocked by this profile", never a permission.
type RouteState struct {
        Route         Route    `json:"route"`
        State         string   `json:"state"`
        Reason        string   `json:"reason,omitempty"`
        UnlockOwner   string   `json:"unlock_owner,omitempty"`
        UnlockOwnerID *id.UUID `json:"unlock_owner_id,omitempty"`
}

// DeriveRouteStates derives the honest per-route states: blocked with a
// concrete reason and unlock owner when the mandatory coverage is missing,
// the fallback is unproven or an OQ-11 assignment stays open; the other
// routes keep living (route-level fail-closed, not profile-level).
func DeriveRouteStates(content ProfileContent) []RouteState {
        states := []RouteState{}
        coverage := map[string]HumanCoverage{}
        for _, row := range content.Coverage {
                coverage[row.Route.String()] = row
        }
        privacyOpen := map[string]bool{}
        for _, row := range content.Privacy {
                if row.InUse && row.OQ11State == OQ11Open {
                        privacyOpen[row.Route.String()] = true
                }
        }
        for _, route := range sortedRoutes(content.WorkScope.Routes) {
                state := RouteState{Route: route, State: RouteStateClear}
                row, covered := coverage[route.String()]
                switch {
                case !covered:
                        state.State = RouteStateBlocked
                        state.Reason = "Обязательное человеческое покрытие маршрута отсутствует."
                        state.UnlockOwner = "Owner/Configurator (владелец настройки)"
                case row.Fallback != "human_callback" && !length(row.FallbackEvidence, 1, 300):
                        state.State = RouteStateBlocked
                        state.Reason = "Fallback «" + row.Fallback + "» не является подтверждённым handoff: нет ссылки на capability evidence."
                        owner := row.ResponseOwnerID
                        state.UnlockOwnerID = &owner
                        state.UnlockOwner = "Response Owner маршрута"
                case privacyOpen[route.String()]:
                        state.State = RouteStateBlocked
                        state.Reason = "Privacy-назначение маршрута в состоянии OQ-11 open: маршрут заблокирован fail-closed до закрытия."
                        owner := row.ResponseOwnerID
                        state.UnlockOwnerID = &owner
                        state.UnlockOwner = "Owner/Data Governance"
                }
                states = append(states, state)
        }
        return states
}

// MatrixCellState is one derived cell of the coverage matrix: covered, an
// explicit gap (no cell) or an explicit not admitted.
type MatrixCellState struct {
        Journey          string        `json:"journey"`
        Route            Route         `json:"route"`
        ConversationMode string        `json:"conversation_mode"`
        AutonomyClass    string        `json:"autonomy_class"`
        State            string        `json:"state"`
        Cell             *CoverageCell `json:"cell,omitempty"`
}

// DeriveMatrixCoverage derives the full cartesian product: journeys of the
// pack reference × channels of the allowed routes × declared modes × declared
// classes. A missing cell is an explicit gap; an explicit refusal is shown,
// never excluded; wildcards do not exist.
func DeriveMatrixCoverage(content ProfileContent, journeys []string) []MatrixCellState {
        cells := map[string]CoverageCell{}
        for _, cell := range content.Matrix {
                cells[cell.Journey+" · "+cell.Route.String()+" · "+cell.ConversationMode+" · "+cell.AutonomyClass] = cell
        }
        out := []MatrixCellState{}
        for _, journey := range sortedCopy(journeys) {
                for _, route := range sortedRoutes(content.WorkScope.Routes) {
                        for _, mode := range sortedCopy(content.Modes) {
                                for _, class := range sortedCopy(content.AutonomyClasses) {
                                        key := journey + " · " + route.String() + " · " + mode + " · " + class
                                        state := MatrixCellState{Journey: journey, Route: route, ConversationMode: mode, AutonomyClass: class, State: MatrixCellGap}
                                        if cell, ok := cells[key]; ok {
                                                cellCopy := cell
                                                state.Cell = &cellCopy
                                                if cell.NotAdmitted {
                                                        state.State = MatrixCellNotAdmitted
                                                } else {
                                                        state.State = MatrixCellCovered
                                                }
                                        }
                                        out = append(out, state)
                                }
                        }
                }
        }
        return out
}

// ThresholdState is the derived state of one declared gate: pending baseline
// until the evidence owner decides, with the owner, deadline and fail-closed
// fallback; a decision is a human assertion with references — the system
// never claims a machine verification against a controlled baseline.
type ThresholdState struct {
        GateID               string                `json:"gate_id"`
        State                string                `json:"state"`
        Value                string                `json:"value,omitempty"`
        EvidenceOwnerID      id.UUID               `json:"evidence_owner_id"`
        DecisionDeadline     string                `json:"decision_deadline"`
        FailClosedFallback   string                `json:"fail_closed_fallback"`
        AllowRoleCoincidence bool                  `json:"allow_role_coincidence"`
        LastDecision         *GateThresholdVersion `json:"last_decision,omitempty"`
        Note                 string                `json:"note"`
}

// DeriveThresholdStates derives every declared gate's threshold state from
// the threshold history (all versions, any order — the latest per gate wins).
func DeriveThresholdStates(content ProfileContent, thresholds []GateThresholdVersion) []ThresholdState {
        latest := map[string]GateThresholdVersion{}
        for _, threshold := range thresholds {
                known, ok := latest[threshold.GateID]
                if !ok || threshold.ThresholdVersion > known.ThresholdVersion {
                        latest[threshold.GateID] = threshold
                }
        }
        out := []ThresholdState{}
        gates := append([]GateDeclaration(nil), content.Gates...)
        sort.SliceStable(gates, func(i, j int) bool { return gates[i].GateID < gates[j].GateID })
        for _, gate := range gates {
                state := ThresholdState{
                        GateID: gate.GateID, State: ThresholdStatePendingBaseline,
                        EvidenceOwnerID: gate.EvidenceOwnerID, DecisionDeadline: gate.DecisionDeadline,
                        FailClosedFallback: gate.FailClosedFallback, AllowRoleCoincidence: gate.AllowRoleCoincidence,
                        Note: "Числового порога нет: до решения evidence owner значение честно отсутствует (pending baseline); система не подставляет значения по умолчанию.",
                }
                if decision, ok := latest[gate.GateID]; ok {
                        decisionCopy := decision
                        state.State = ThresholdStateDecided
                        state.Value = decision.Value
                        state.LastDecision = &decisionCopy
                        state.Note = "Порог утверждён человеком со ссылками; машинная сверка с controlled baseline не заявляется (поверхностей baseline-данных в системе нет)."
                }
                out = append(out, state)
        }
        return out
}

// ProfileApplicability is the derived applicability of the approved profile.
type ProfileApplicability struct {
        State  string `json:"state"`
        Reason string `json:"reason"`
}

// DeriveProfileApplicability derives the honest applicability: a newly sealed
// manifest makes the approved profile stale for the new version (a new draft
// and a new approval are required — never an on-the-spot migration); draft
// edits after approval surface as unapproved changes while the approved
// version stays immutable.
func DeriveProfileApplicability(approved *ProfileVersion, draft *ProfileDraft, latestManifest *ManifestLink) ProfileApplicability {
        if approved == nil {
                return ProfileApplicability{State: ProfileApplicabilityNone, Reason: "Утверждённой версии профиля ещё нет: черновик редактируется, допуск не затрагивается."}
        }
        if latestManifest != nil && !approved.Manifest.Equal(*latestManifest) {
                return ProfileApplicability{State: ProfileApplicabilityStale, Reason: "Запечатан новый манифест: утверждённый профиль неприменим к новой конфигурационной версии (stale) — нужен новый черновик и новое утверждение; на месте профиль не мигрируется."}
        }
        if draft != nil && draft.Manifest.Equal(approved.Manifest) && draft.ApprovedRevision != nil && draft.Revision > *draft.ApprovedRevision {
                return ProfileApplicability{State: ProfileApplicabilityUnapproved, Reason: "Черновик изменён после утверждения: есть неутверждённые изменения; утверждённая версия и её digest неизменны, затронутые readiness/evidence состояния — stale до повторного утверждения."}
        }
        return ProfileApplicability{State: ProfileApplicabilityCurrent, Reason: "Утверждённая версия применима к указанной конфигурационной версии манифеста (профиль — правила работы, а не допуск трафика)."}
}
