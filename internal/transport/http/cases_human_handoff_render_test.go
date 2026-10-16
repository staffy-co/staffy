package httptransport

import (
	"bytes"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"text/template"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	staffyweb "github.com/artemiosu/staffy/web"
)

// TestCaseHandoffPanelRenders is the Story 2.13 render smoke test of the
// «Запрос человека» section: the template executes with the POPULATED
// handoff rows (the request rows with the event histories, the assignment
// card of the acting assignee, the dictionaries of the forms) — a missing
// field of a range body fails the render here, BEFORE the journeys of
// step-04 (the 2-12 step-04 QA lesson: the history row of the degradation
// panel referenced a field the projection never carried — the page render
// broke on the first recorded row and truncated the case page).
func TestCaseHandoffPanelRenders(t *testing.T) {
	templates, err := template.New("staffy").Funcs(template.FuncMap{
		"hasRole":   func(roles []domain.Role, role string) bool { return slices.Contains(roles, domain.Role(role)) },
		"hasScope":  func(scopes []domain.Scope, scope string) bool { return slices.Contains(scopes, domain.Scope(scope)) },
		"joinLines": func(values []string) string { return strings.Join(values, "\n") },
		"contains":  func(values []string, value string) bool { return slices.Contains(values, value) },
		"statusNote": func(status, auth, transport, environment string) string {
			return connectionStatusText(status, auth, transport, environment)
		},
		"stepStatus": func(status string) string {
			return map[string]string{"complete": "Готово", "incomplete": "Не завершено", "blocked": "Заблокировано", "requires review": "Требует проверки"}[status]
		},
	}).ParseFS(staffyweb.Assets, "templates/*.html")
	if err != nil {
		t.Fatalf("the case template does not parse: %v", err)
	}
	view := caseDetailPageView{
		CaseID:               "01994f64-6c00-7000-8000-000000000010",
		EmployeeID:           "01994f64-6c00-7000-8000-000000000011",
		CanManageWork:        true,
		Flash:                "",
		HandoffsPanelText:    "Запрос человека — запись жизненного цикла обращения, не факт и не доказательство.",
		HandoffCurrentText:   "Запрос человека: Назначен: ожидание подтверждения специалистом. (слово: assigned, запрос №1)",
		HandoffClientText:    "Клиентский статус: Специалист назначен и ещё не подтвердил принятие обращения. (слово: human_assigned)",
		HasHandoffs:          true,
		HandoffCardForActor:  true,
		HandoffCardStateText: casesDomain.HandoffStateText(casesDomain.HandoffStateAssigned),
		HandoffCardDeadline:  "2026-10-15 12:00",
		HandoffCardAssignee:  "01994f64-6c00-7000-8000-000000000021",
		HandoffRoleWords: []workOption{
			{Value: casesDomain.HandoffTargetRoleOperator, Text: "operator — рабочий оператор"},
			{Value: casesDomain.HandoffTargetRoleSupervision, Text: "supervision — надзор"},
		},
		HandoffDeadlineOptions: []workOption{
			{Value: casesDomain.HandoffAck15m, Text: casesDomain.HandoffAckDeadlineText(casesDomain.HandoffAck15m)},
			{Value: casesDomain.HandoffAck1h, Text: casesDomain.HandoffAckDeadlineText(casesDomain.HandoffAck1h)},
			{Value: casesDomain.HandoffAck4h, Text: casesDomain.HandoffAckDeadlineText(casesDomain.HandoffAck4h)},
		},
		HandoffResultOptions: []workOption{
			{Value: casesDomain.HandoffResultIssueResolved, Text: casesDomain.HandoffResultText(casesDomain.HandoffResultIssueResolved)},
			{Value: casesDomain.HandoffResultHandedBack, Text: casesDomain.HandoffResultText(casesDomain.HandoffResultHandedBack)},
		},
		HandoffRequestsAnchor: "1",
		HandoffEventsAnchor:   "2",
		HandoffRequestRows: []handoffRequestRow{{
			RequestNo:    "1",
			ReasonText:   casesDomain.HandoffReasonText(casesDomain.HandoffReasonClientRequest),
			State:        casesDomain.HandoffStateAssigned,
			StateText:    casesDomain.HandoffStateText(casesDomain.HandoffStateAssigned),
			ClientText:   casesDomain.HandoffClientStatusText(casesDomain.HandoffClientAssigned),
			ActiveText:   "открыт",
			RolesText:    "operator, supervision",
			DeadlineText: casesDomain.HandoffAckDeadlineText(casesDomain.HandoffAck15m),
			TurnRef:      "01994f64-6c00-7000-8000-000000000031",
			Assignee:     "01994f64-6c00-7000-8000-000000000021",
			Backup:       "01994f64-6c00-7000-8000-000000000022",
			DeadlineAt:   "2026-10-15 12:00",
			CreatedAt:    "2026-10-15 11:00",
			HasRound:     true,
			HasEvents:    true,
			Events: []handoffEventRow{
				{KindText: "Назначение специалиста: ожидание подтверждения.", MaterialText: "специалист, резерв, дедлайн", ActorID: "01994f64-6c00-7000-8000-000000000001", CreatedAt: "2026-10-15 11:30"},
				{KindText: "Отказ специалиста: ожидание нового назначения.", MaterialText: "владелец не изменился", ActorID: "01994f64-6c00-7000-8000-000000000021", CreatedAt: "2026-10-15 11:45"},
			},
		}},
		Ownership: ownershipPanelView{
			CurrentEpoch:  "1",
			Epoch:         "1",
			MemberOptions: []workOption{{Value: "01994f64-6c00-7000-8000-000000000021", Text: "operator one"}, {Value: "01994f64-6c00-7000-8000-000000000022", Text: "operator two"}},
		},
		Mode:         modePanelView{CurrentVersion: "2"},
		EventOptions: []workOption{{Value: "01994f64-6c00-7000-8000-000000000031", Text: "web_chat · 2026-10-15 11:00"}},
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the populated handoff panel: %v", err)
	}
	rendered := page.String()
	for _, anchor := range []string{
		"Запрос человека — жизненный цикл передачи",
		"Передача ждёт вашего подтверждения",
		"Записать просьбу клиента",
		"Назначить раунд",
		"Записать отмену",
		"Вернуть цифровому сотруднику",
		"human-requests/acknowledgement",
		"human-requests/decline",
		"human-requests/cancellation",
		"human-requests/return",
		"human-requests/assignment",
	} {
		if !strings.Contains(rendered, anchor) {
			t.Errorf("the rendered case page carries the %q anchor of the handoff panel", anchor)
		}
	}
	_ = slog.Default()
}
