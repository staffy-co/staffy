package httptransport

import (
	"strconv"
	"strings"
	"time"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
)

// readinessPageView is the flattened, template-friendly projection of the
// Test & Release Panel: every element state, route block, gate and decision
// rendered as text — accessible without color, keyboard-first, no hidden
// aggregates.
type readinessPageView struct {
	CanManage           bool
	ScopeText           string
	Stage               string
	StageText           string
	Status              string
	StatusText          string
	StatusStateText     string
	Elements            []readinessElementRow
	Routes              []readinessRouteRow
	Applicability       string
	ApplicabilityText   string
	ApplicabilityReason string
	Active              *readinessAdmissionRow
	History             []readinessAdmissionRow
	StageChoices        []readinessStageChoice
	HasHistory          bool
}

// readinessElementRow is one checklist element with its state and source.
type readinessElementRow struct {
	Key       string
	Title     string
	State     string
	StateText string
	Reason    string
	Source    string
}

// readinessRouteRow is one derived route admission with every block.
type readinessRouteRow struct {
	Route           string
	Modality        string
	AutonomyClasses string
	State           string
	StateText       string
	Blocks          []readinessBlockRow
}

// readinessBlockRow is one honest blocking of a route.
type readinessBlockRow struct {
	Cause            string
	Reason           string
	UnlockOwner      string
	RequiredEvidence string
	NextStep         string
}

// readinessAdmissionRow is one publication decision of the history.
type readinessAdmissionRow struct {
	DecisionNo    string
	Stage         string
	StageText     string
	Manifest      string
	Profile       string
	RoutesSummary string
	ActorID       string
	ApproverRoles string
	DecidedAt     string
	Digest        string
}

// readinessStageChoice is one selectable stage of the dry-run preview.
type readinessStageChoice struct {
	Stage     string
	StageText string
	Selected  bool
}

// gateInstancesPageView is the projection of the gate registry surface.
type gateInstancesPageView struct {
	CanManage    bool
	Definitions  []readinessDefinitionRow
	Instances    []readinessInstanceRow
	HasInstances bool
}

// readinessDefinitionRow is one dictionary entry.
type readinessDefinitionRow struct {
	ID               string
	Criticality      string
	Roles            string
	Evidence         string
	SafeState        string
	AllowCoincidence string
	RouteScoped      string
}

// readinessInstanceRow is one lifecycle record with the supersedes history.
type readinessInstanceRow struct {
	InstanceID string
	Definition string
	Scope      string
	State      string
	StateText  string
	Approvers  string
	Evidence   string
	Hash       string
	Reason     string
	TestedAt   string
	Expiry     string
	Supersedes string
	RecordedAt string
	ActorID    string
}

// prepareReadinessView converts the service view into the page projection.
func prepareReadinessView(view employeeAPI.ReadinessView) readinessPageView {
	data := readinessPageView{
		CanManage:       view.CanManage,
		ScopeText:       readinessScopeText(view.ViewScope),
		Stage:           view.Stage,
		StageText:       readinessStageText(view.Stage),
		Status:          view.Report.Status,
		StatusText:      view.Report.StatusText,
		StatusStateText: readinessStatusText(view.Report.Status),
		Elements:        []readinessElementRow{},
		Routes:          []readinessRouteRow{},
		History:         []readinessAdmissionRow{},
		StageChoices:    []readinessStageChoice{},
	}
	for _, element := range view.Report.Elements {
		data.Elements = append(data.Elements, readinessElementRow{
			Key: element.Key, Title: element.Title, State: element.State,
			StateText: readinessElementStateText(element.State), Reason: element.Reason, Source: element.Source,
		})
	}
	for _, route := range view.Report.Routes {
		row := readinessRouteRow{
			Route: route.Route.String(), Modality: route.Modality,
			AutonomyClasses: strings.Join(route.AutonomyClasses, ", "),
			State:           route.State, StateText: readinessRouteStateText(route.State),
			Blocks: []readinessBlockRow{},
		}
		for _, block := range route.Reasons {
			row.Blocks = append(row.Blocks, readinessBlockRow{
				Cause: block.Cause, Reason: block.Reason, UnlockOwner: block.UnlockOwner,
				RequiredEvidence: block.RequiredEvidence, NextStep: block.NextStep,
			})
		}
		data.Routes = append(data.Routes, row)
	}
	data.Applicability = view.Applicability.State
	data.ApplicabilityText = readinessApplicabilityText(view.Applicability.State)
	data.ApplicabilityReason = view.Applicability.Reason
	if view.Active != nil {
		active := readinessAdmissionRowOf(*view.Active)
		data.Active = &active
	}
	for _, admission := range view.History {
		data.History = append(data.History, readinessAdmissionRowOf(admission))
	}
	data.HasHistory = len(data.History) > 0
	for _, stage := range employeeDomain.StageOrder {
		data.StageChoices = append(data.StageChoices, readinessStageChoice{
			Stage: stage, StageText: readinessStageText(stage), Selected: stage == view.Stage,
		})
	}
	return data
}

// readinessAdmissionRowOf flattens one decision.
func readinessAdmissionRowOf(admission employeeDomain.ReleaseAdmission) readinessAdmissionRow {
	admitted, blocked := 0, 0
	for _, route := range admission.Routes {
		if route.State == employeeDomain.RouteAdmitted {
			admitted++
		} else {
			blocked++
		}
	}
	return readinessAdmissionRow{
		DecisionNo:    strconv.FormatInt(admission.DecisionNo, 10),
		Stage:         admission.Stage,
		StageText:     readinessStageText(admission.Stage),
		Manifest:      "manifest " + admission.Manifest.ManifestID.String() + " @ config v" + strconv.FormatInt(admission.Manifest.ConfigVersion, 10) + ", digest " + admission.Manifest.Digest,
		Profile:       "profile v" + strconv.FormatInt(admission.ProfileVersion, 10) + ", digest " + admission.ProfileDigest,
		RoutesSummary: "допущено " + strconv.Itoa(admitted) + ", заблокировано " + strconv.Itoa(blocked),
		ActorID:       admission.ActorID.String(),
		ApproverRoles: strings.Join(admission.ApproverRoles, ", "),
		DecidedAt:     readinessFormatTimestamp(admission.DecidedAt),
		Digest:        admission.Digest,
	}
}

// prepareGateInstancesView converts the gate registry view.
func prepareGateInstancesView(view employeeAPI.GateInstancesView) gateInstancesPageView {
	data := gateInstancesPageView{
		CanManage:   view.CanManage,
		Definitions: []readinessDefinitionRow{},
		Instances:   []readinessInstanceRow{},
	}
	for _, definition := range view.Definitions {
		row := readinessDefinitionRow{
			ID: definition.ID, Criticality: definition.Criticality,
			Roles:    strings.Join(definition.RequiredApproverRoles, " + "),
			Evidence: definition.EvidenceRequirement, SafeState: definition.SafeState,
		}
		if definition.AllowRoleCoincidence {
			row.AllowCoincidence = "разрешено (allow_role_coincidence)"
		} else {
			row.AllowCoincidence = "различные принципалы обязательны"
		}
		if definition.RouteScoped {
			row.RouteScoped = "route-scoped (инстанс несёт точный маршрут)"
		} else {
			row.RouteScoped = "tenant-уровень (без scope маршрута)"
		}
		data.Definitions = append(data.Definitions, row)
	}
	for _, instance := range view.Instances {
		scope := "tenant"
		if instance.Route != nil {
			scope = "маршрут " + instance.Route.String()
		}
		if instance.Modality != "" {
			scope += ", modality " + instance.Modality
		}
		approvers := []string{}
		for _, approval := range instance.ApprovedBy {
			approvers = append(approvers, approval.ParticipantID.String()+" ("+approval.Role+")")
		}
		row := readinessInstanceRow{
			InstanceID: instance.InstanceID.String(),
			Definition: instance.DefinitionID,
			Scope:      scope,
			State:      instance.State,
			StateText:  employeeAPI.GateInstanceStateText(instance.State),
			Approvers:  strings.Join(approvers, "; "),
			Evidence:   strings.Join(instance.EvidenceRefs, "; "),
			Hash:       instance.EvidenceHash,
			Reason:     instance.NotApplicableReason,
			Expiry:     instance.ExpiryTrigger,
			RecordedAt: readinessFormatTimestamp(instance.RecordedAt),
			ActorID:    instance.ActorID.String(),
		}
		if instance.TestedAt != nil {
			row.TestedAt = readinessFormatTimestamp(*instance.TestedAt)
		}
		if instance.SupersedesInstanceID != nil {
			row.Supersedes = instance.SupersedesInstanceID.String()
		}
		data.Instances = append(data.Instances, row)
	}
	data.HasInstances = len(data.Instances) > 0
	return data
}

func readinessScopeText(scope string) string {
	if scope == employeeAPI.ReadinessScopeManage {
		return "configuration — просмотр и команды"
	}
	return "review/audit — только просмотр, команды недоступны"
}

func readinessStageText(stage string) string {
	switch stage {
	case employeeDomain.StageSynthetic:
		return "synthetic (фикстуры, без реальных клиентов)"
	case employeeDomain.StageClosedAccountTest:
		return "internal · closed_account_test (закрытые тестовые аккаунты)"
	case employeeDomain.StageControlledRealCustomer:
		return "controlled_real_customer (реальные клиенты в когорте)"
	case employeeDomain.StageLive:
		return "live (обычный допущенный трафик)"
	}
	return stage
}

func readinessStatusText(status string) string {
	switch status {
	case employeeDomain.ReadinessReady:
		return "ready"
	case employeeDomain.ReadinessBlocked:
		return "blocked"
	default:
		return "incomplete"
	}
}

func readinessElementStateText(state string) string {
	switch state {
	case employeeDomain.ElementPass:
		return "готов"
	case employeeDomain.ElementMissing:
		return "отсутствует"
	case employeeDomain.ElementNotApplicable:
		return "неприменим"
	case employeeDomain.ElementLinkedNotExecuted:
		return "связано, не исполнено"
	default:
		return "заблокирован"
	}
}

func readinessRouteStateText(state string) string {
	if state == employeeDomain.RouteAdmitted {
		return "admitted"
	}
	return "blocked"
}

func readinessApplicabilityText(state string) string {
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

// readinessFormatTimestamp renders one honest timestamp.
func readinessFormatTimestamp(t time.Time) string {
	return t.Format("02.01.2006 15:04 UTC")
}
