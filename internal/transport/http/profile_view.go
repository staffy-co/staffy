package httptransport

import (
        "strconv"
        "strings"
        "time"

        employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
        employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
)

// profilePageView is the flattened, template-friendly projection of the
// operating profile page: every status, unit, gap, owner and reason rendered
// as text, the matrix as rows of one honest table.
type profilePageView struct {
        CanManage       bool
        ScopeText       string
        Draft           *profileDraftRow
        Readiness       []profileProblemRow
        Approved        *profileApprovedRow
        Applicability   profileApplicabilityRow
        RouteStates     []profileRouteRow
        Matrix          []profileMatrixRow
        Thresholds      []profileThresholdRow
        LatestManifest  string
        HasMatrix       bool
        MatrixAxisText  string
        ContentJSON     string
        ContentRows     int
        ManifestID      string
        ConfigVersion   string
        ManifestDigest  string
        ExpectedRevision string
}

// profileDraftRow carries the mutable draft with its exact manifest link.
type profileDraftRow struct {
        Revision        string
        ApprovedAt      string
        ManifestID      string
        ConfigVersion   string
        ManifestDigest  string
        UpdatedAt       string
        HasUnapproved   bool
}

// profileApprovedRow is the immutable approved version.
type profileApprovedRow struct {
        Version         string
        Digest          string
        ManifestID      string
        ConfigVersion   string
        ManifestDigest  string
        Description     string
        ApproverID      string
        ApproverRoles   string
        EvidenceRefs    string
        ApprovedAt      string
}

// profileApplicabilityRow states the honest applicability of the approved
// profile for the current manifest.
type profileApplicabilityRow struct {
        State     string
        StateText string
        Reason    string
}

// profileRouteRow is one derived route state.
type profileRouteRow struct {
        Route       string
        State       string
        StateText   string
        Reason      string
        UnlockOwner string
}

// profileMatrixRow is one derived cell of the coverage matrix: covered, an
// explicit gap or an explicit not admitted — nothing is hidden.
type profileMatrixRow struct {
        Journey          string
        Route            string
        ConversationMode string
        AutonomyClass    string
        State            string
        StateText        string
        Reason           string
        EvalRef          string
        Gates            string
        HumanFallback    string
}

// profileThresholdRow is one derived gate threshold state.
type profileThresholdRow struct {
        GateID               string
        State                string
        StateText            string
        Value                string
        EvidenceOwnerID      string
        DecisionDeadline     string
        FailClosedFallback   string
        AllowRoleCoincidence string
        Note                 string
        LastDecision         string
}

// profileProblemRow is one readiness/validation problem leading to a
// concrete section and field.
type profileProblemRow struct {
        Section string
        Field   string
        Reason  string
        Kind    string
        KindText string
}

// prepareProfileView converts the service view into the page projection.
func prepareProfileView(view employeeAPI.ProfileView) profilePageView {
        data := profilePageView{
                CanManage:      view.CanManage,
                ScopeText:      profileScopeText(view.ViewScope),
                Readiness:      []profileProblemRow{},
                RouteStates:    []profileRouteRow{},
                Matrix:         []profileMatrixRow{},
                Thresholds:     []profileThresholdRow{},
        }
        if view.Draft != nil {
                draft := profileDraftRow{
                        Revision:      strconv.FormatInt(view.Draft.Revision, 10),
                        ManifestID:    view.Draft.Manifest.ManifestID.String(),
                        ConfigVersion: strconv.FormatInt(view.Draft.Manifest.ConfigVersion, 10),
                        ManifestDigest: view.Draft.Manifest.Digest,
                        UpdatedAt:     profileFormatTimestamp(view.Draft.UpdatedAt),
                }
                if view.Draft.ApprovedRevision != nil && view.Draft.Revision > *view.Draft.ApprovedRevision {
                        draft.HasUnapproved = true
                }
                data.Draft = &draft
                if raw, err := employeeDomain.CanonicalProfileJSON(view.Draft.Content); err == nil {
                        data.ContentJSON = string(raw)
                        data.ContentRows = strings.Count(data.ContentJSON, "\n") + 3
                }
                data.ManifestID = draft.ManifestID
                data.ConfigVersion = draft.ConfigVersion
                data.ManifestDigest = draft.ManifestDigest
                data.ExpectedRevision = draft.Revision
        }
        for _, problem := range view.Readiness {
                data.Readiness = append(data.Readiness, profileProblemRow{
                        Section: problem.Section, Field: problem.Field, Reason: problem.Reason,
                        Kind: problem.Kind, KindText: profileProblemKindText(problem.Kind),
                })
        }
        if view.Approved != nil {
                approved := profileApprovedRow{
                        Version:       strconv.FormatInt(view.Approved.Version, 10),
                        Digest:        view.Approved.Digest,
                        ManifestID:    view.Approved.Manifest.ManifestID.String(),
                        ConfigVersion: strconv.FormatInt(view.Approved.Manifest.ConfigVersion, 10),
                        ManifestDigest: view.Approved.Manifest.Digest,
                        Description:   view.Approved.Description,
                        ApproverID:    view.Approved.ApproverID.String(),
                        ApproverRoles: strings.Join(view.Approved.ApproverRoles, ", "),
                        EvidenceRefs:  strings.Join(view.Approved.EvidenceRefs, "; "),
                        ApprovedAt:    profileFormatTimestamp(view.Approved.ApprovedAt),
                }
                data.Approved = &approved
        }
        data.Applicability = profileApplicabilityRow{
                State:     view.Applicability.State,
                StateText: profileApplicabilityText(view.Applicability.State),
                Reason:    view.Applicability.Reason,
        }
        for _, state := range view.RouteStates {
                data.RouteStates = append(data.RouteStates, profileRouteRow{
                        Route: state.Route.String(), State: state.State, StateText: profileRouteStateText(state.State),
                        Reason: state.Reason, UnlockOwner: state.UnlockOwner,
                })
        }
        for _, cell := range view.Matrix {
                row := profileMatrixRow{
                        Journey: cell.Journey, Route: cell.Route.String(),
                        ConversationMode: cell.ConversationMode, AutonomyClass: cell.AutonomyClass,
                        State: cell.State, StateText: profileMatrixStateText(cell.State),
                }
                if cell.Cell != nil {
                        row.Reason = cell.Cell.NotAdmittedReason
                        row.EvalRef = cell.Cell.EvalRef
                        row.Gates = strings.Join(cell.Cell.Gates, ", ")
                        row.HumanFallback = cell.Cell.HumanFallback
                }
                data.Matrix = append(data.Matrix, row)
        }
        data.HasMatrix = len(view.Matrix) > 0
        if data.HasMatrix {
                data.MatrixAxisText = "строки — journey и маршрут, столбцы — Conversation Mode и Autonomy Class"
        }
        for _, state := range view.Thresholds {
                row := profileThresholdRow{
                        GateID: state.GateID, State: state.State, StateText: profileThresholdStateText(state.State),
                        Value: state.Value, EvidenceOwnerID: state.EvidenceOwnerID.String(),
                        DecisionDeadline: state.DecisionDeadline, FailClosedFallback: state.FailClosedFallback,
                        Note: state.Note,
                }
                if state.AllowRoleCoincidence {
                        row.AllowRoleCoincidence = "разрешено (allow_role_coincidence)"
                } else {
                        row.AllowRoleCoincidence = "не разрешено"
                }
                if state.LastDecision != nil {
                        row.LastDecision = "версия решения " + strconv.FormatInt(state.LastDecision.ThresholdVersion, 10) +
                                " от " + profileFormatTimestamp(state.LastDecision.DecidedAt) + ": " + state.LastDecision.Decision
                }
                data.Thresholds = append(data.Thresholds, row)
        }
        if view.LatestManifest != nil {
                data.LatestManifest = "manifest " + view.LatestManifest.ManifestID.String() +
                        " @ config v" + strconv.FormatInt(view.LatestManifest.ConfigVersion, 10)
                // The creation form is prefilled from the latest SEALED manifest row:
                // honest values from the stored row, confirmed by the submit — never
                // invented defaults.
                if view.Draft == nil {
                        data.ManifestID = view.LatestManifest.ManifestID.String()
                        data.ConfigVersion = strconv.FormatInt(view.LatestManifest.ConfigVersion, 10)
                        data.ManifestDigest = view.LatestManifest.Digest
                        data.ExpectedRevision = "0"
                }
        }
        return data
}

func profileScopeText(scope string) string {
        if scope == employeeAPI.ProfileScopeManage {
                return "configuration — просмотр и управление"
        }
        return "review/audit — только просмотр, управление недоступно"
}

func profileProblemKindText(kind string) string {
        switch kind {
        case employeeDomain.ProfileProblemInvalid:
                return "невалидное значение — сохранение отклонено"
        case employeeDomain.ProfileProblemMissing:
                return "обязательный раздел не заполнен — блокирует утверждение"
        case employeeDomain.ProfileProblemAdvisory:
                return "состояние маршрута — блокирует маршрут, не утверждение"
        }
        return kind
}

func profileApplicabilityText(state string) string {
        switch state {
        case employeeDomain.ProfileApplicabilityCurrent:
                return "применим"
        case employeeDomain.ProfileApplicabilityStale:
                return "неприменим (stale)"
        case employeeDomain.ProfileApplicabilityUnapproved:
                return "есть неутверждённые изменения"
        default:
                return "утверждённой версии нет"
        }
}

func profileRouteStateText(state string) string {
        if state == employeeDomain.RouteStateBlocked {
                return "blocked"
        }
        return "clear"
}

func profileMatrixStateText(state string) string {
        switch state {
        case employeeDomain.MatrixCellCovered:
                return "покрыто"
        case employeeDomain.MatrixCellNotAdmitted:
                return "не допущено"
        default:
                return "gap — ячейка не заполнена"
        }
}

func profileThresholdStateText(state string) string {
        if state == employeeDomain.ThresholdStateDecided {
                return "порог утверждён"
        }
        return "pending baseline"
}

// profileFormatTimestamp renders one honest timestamp.
func profileFormatTimestamp(t time.Time) string {
        return t.Format("02.01.2006 15:04 UTC")
}
