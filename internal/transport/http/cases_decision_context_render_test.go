package httptransport

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"text/template"

	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	staffyweb "github.com/staffy-co/staffy/web"
)

// decisionContextRenderTemplates parses the case template with the
// production function set (the handoff render smoke precedent).
func decisionContextRenderTemplates(t *testing.T) *template.Template {
	t.Helper()
	templates, err := template.New("staffy").Funcs(template.FuncMap{
		"hasRole":   func(roles []domain.Role, role string) bool { return containsRole(roles, role) },
		"hasScope":  func(scopes []domain.Scope, scope string) bool { return containsScope(scopes, scope) },
		"joinLines": func(values []string) string { return strings.Join(values, "\n") },
		"contains":  func(values []string, value string) bool { return containsWord(values, value) },
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
	return templates
}

func containsRole(roles []domain.Role, role string) bool {
	for _, candidate := range roles {
		if string(candidate) == role {
			return true
		}
	}
	return false
}

func containsScope(scopes []domain.Scope, scope string) bool {
	for _, candidate := range scopes {
		if string(candidate) == scope {
			return true
		}
	}
	return false
}

func containsWord(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// populatedDecisionContextPanel builds the fully populated panel of the
// «Контекст передачи» section: every block carries material (the
// open-handoff ask, the active work items, the three fact groups, the
// observations with the provider technique, the deliveries, the
// commitments with the honest receipt line, the preferences, the
// recommendation projection and the stale summary with the claims).
func populatedDecisionContextPanel() decisionContextPanelView {
	return decisionContextPanelView{
		Present:   true,
		PanelText: "Контекст передачи — производное чтение собранного материала, не новая истина.",
		Handoff: decisionContextHandoffPanel{
			Present: true, ReasonText: casesDomain.HandoffReasonText(casesDomain.HandoffReasonClientRequest),
			State: casesDomain.HandoffStateAssigned, StateText: casesDomain.HandoffStateText(casesDomain.HandoffStateAssigned),
			TurnRef:         "01994f64-6c00-7000-8000-000000000031",
			AckDeadlineText: casesDomain.HandoffAckDeadlineText(casesDomain.HandoffAck1h),
			DeadlineText:    "17.10.2026 12:00:00 UTC",
		},
		HasWorkItems: true,
		WorkItems: []decisionContextWorkItemRow{
			{ID: "01994f64-6c00-7000-8000-000000000040", Title: "Доставка", Type: "delivery", State: "in_work"},
		},
		Mode:      decisionContextModePanel{Present: true, Mode: "human_requested", Version: "3"},
		Assurance: decisionContextAssurancePanel{IdentityLine: "basic (operator_relay)", AuthorityLine: "нет записи"},
		Owner:     decisionContextOwnerPanel{Present: true, Kind: "staffy", Ref: "01994f64-6c00-7000-8000-000000000011", Epoch: "2"},
		Facts: decisionContextFactsPanel{
			HasConfirmed: true,
			Confirmed: []decisionContextFactRow{{
				RowID: "01994f64-6c00-7000-8000-00000000ff01", MaterialKey: "address", Status: "confirmed",
				StatusText: casesDomain.ContextFactStatusText(casesDomain.ContextFactConfirmed), Version: "2",
				ProvenanceText: "client_event 01994f64-6c00-7000-8000-000000000031 · web_chat", ObservedText: "17.10.2026 10:00:00 UTC",
			}},
			HasUnconfirmed: true,
			Unconfirmed: []decisionContextFactRow{{
				RowID: "01994f64-6c00-7000-8000-00000000ff02", MaterialKey: "urgency", Status: "unconfirmed",
				StatusText: casesDomain.ContextFactStatusText(casesDomain.ContextFactUnconfirmed), Version: "1",
				ProvenanceText: "client_event 01994f64-6c00-7000-8000-000000000032 · web_chat", ObservedText: "17.10.2026 10:05:00 UTC",
			}},
			HasConflicting: true,
			Conflicting: []decisionContextFactRow{{
				RowID: "01994f64-6c00-7000-8000-00000000ff03", MaterialKey: "service_kind", Status: "conflicting",
				StatusText: casesDomain.ContextFactStatusText(casesDomain.ContextFactConflicting), Version: "3",
				ProvenanceText: "client_event 01994f64-6c00-7000-8000-000000000033 · web_chat", ObservedText: "17.10.2026 10:10:00 UTC",
			}},
		},
		HasObservations: true,
		Observations: []decisionContextObservationRow{{
			WorkItemID: "01994f64-6c00-7000-8000-000000000040", MaterialKey: "delivery_slot", OwningSystem: "calendar",
			ObservedText: "17.10.2026 09:00:00 UTC", Freshness: "fresh", ConflictRule: "prefer_newer", Evidence: "observed",
			TechniqueText: "проекция: slot=14; внешний id: cal-42; внешняя версия: 3",
		}},
		HasDeliveries: true,
		Deliveries: []decisionContextDeliveryRow{{
			ReplyID: "01994f64-6c00-7000-8000-00000000ee01", State: "delivered", LastKind: "provider_receipt", LastEventText: "17.10.2026 09:30:00 UTC",
		}},
		HasCommitments: true,
		Commitments: []decisionContextCommitmentRow{{
			ID: "01994f64-6c00-7000-8000-00000000cc01", Version: "2", Status: "offered",
			ValidUntilText: "18.10.2026 12:00:00 UTC", TermsKnownText: "scope; deadline", ReceiptText: casesDomain.ReceiptAbsentText,
		}},
		Preferences: decisionContextPreferencesPanel{
			AccommodationsText: "slowed_pace", AlternativesText: "web_chat",
			HasModalities: true,
			Modalities:    []decisionContextModalityRow{{Key: "address", Word: "media_photo"}},
		},
		Recommendation: decisionContextRecommendationPanel{
			Present: true, StepsText: "step_human_handoff", Reason: "client_request",
			Checks:       []decisionContextCheckRow{{Key: "address", State: "ask"}},
			DeadlineText: "17.10.2026 12:00:00 UTC", AckDeadlineText: casesDomain.HandoffAckDeadlineText(casesDomain.HandoffAck1h),
			Mode: "human_requested", ProjectionText: casesDomain.RecommendationProjectionText,
		},
		Summary: decisionContextSummaryPanel{
			Present: true, No: "1", Kind: "ai_summary", MarkingText: casesDomain.AIMarkingText,
			Text:      "Клиент ожидает звонка сегодня; адрес подтверждён сообщением клиента.",
			Staleness: casesDomain.ContextStalenessStale, Stale: true,
			StaleText: casesDomain.ContextStaleText, StalenessCountersText: "новых событий: 1; владелец сменился",
			HasClaims: true,
			Claims: []decisionContextClaimRow{
				{RefType: "fact_version", RefID: "01994f64-6c00-7000-8000-00000000ff01", Text: "адрес подтверждён сообщением клиента"},
				{RefType: "observation", RefID: "01994f64-6c00-7000-8000-000000000040", Text: "слот доставки назначен системой"},
				{Disputed: true, Text: "Клиент ожидает звонка сегодня"},
			},
		},
	}
}

// TestCaseDecisionContextPanelRenders is the Story 2.14 render smoke
// test of the «Контекст передачи» section: the template executes with
// the POPULATED context blocks (the open-handoff ask, the three fact
// groups, the observations with the provider technique behind the
// disclosure, the stale summary with the claim rows and the
// recommendation projection) — a missing field of a range body fails
// the render here, BEFORE the journeys of step-04 (the 2-12 step-04 QA
// lesson).
func TestCaseDecisionContextPanelRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:          "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID:      "01994f64-6c00-7000-8000-000000000011",
		DecisionContext: populatedDecisionContextPanel(),
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the populated decision context: %v", err)
	}
	rendered := page.String()
	for _, anchor := range []string{
		"Контекст передачи",
		"id=\"decision-context\"",
		"Обновить контекст",
		"Что происходит",
		"Факты",
		"Наблюдения систем",
		"Обязательства",
		"Предпочтения клиента",
		"Сводка AI",
		"Рекомендуемый шаг",
		"Квитанции нет",
		casesDomain.AIMarkingText,
		casesDomain.ContextStaleText,
		"новых событий: 1; владелец сменился",
		"id=\"ctx-fact-01994f64-6c00-7000-8000-00000000ff01\"",
		"href=\"#ctx-fact-01994f64-6c00-7000-8000-00000000ff01\"",
		"href=\"#ctx-observations\"",
		"помечено спорным",
		"Техника провайдера",
		"проекция: slot=14",
		"принятие каналом ≠ доставка ≠ деловое завершение",
		casesDomain.RecommendationProjectionText,
	} {
		if !strings.Contains(rendered, anchor) {
			t.Errorf("the rendered case page carries the %q anchor of the decision context section", anchor)
		}
	}
}

// TestCaseDecisionContextAbsentRendersByteForByte is the control branch
// of the slice: a case WITHOUT the context material (no open handoff,
// no recorded summary) renders the page with NO section — the honest
// absence emits nothing between the handoff section and the Work
// Control Card heading, byte-for-byte the unchanged boundary (the
// {{if}} guard carries the whole block including its newlines).
func TestCaseDecisionContextAbsentRendersByteForByte(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:     "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID: "01994f64-6c00-7000-8000-000000000011",
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render without the decision context: %v", err)
	}
	rendered := page.String()
	for _, absent := range []string{
		"Контекст передачи",
		"id=\"decision-context\"",
		"Обновить контекст",
		"Сводка AI",
		"Рекомендуемый шаг",
	} {
		if strings.Contains(rendered, absent) {
			t.Errorf("the page without the context material must NOT carry the %q marker of the section", absent)
		}
	}
	// The byte-for-byte boundary pin: the unchanged page emits exactly
	// one blank line between the closed handoff section and the Work
	// Control Card heading — the guard block contributes NOTHING when
	// the material is absent (the honest pre-change layout).
	if !strings.Contains(rendered, "</section>\n\n<h2>Work Control Card — рабочие элементы</h2>") {
		t.Errorf("the absent context keeps the unchanged section boundary byte-for-byte")
	}
	if strings.Contains(rendered, "</section>\n\n\n<h2>Work Control Card") {
		t.Errorf("the absent context must not add even one newline to the unchanged boundary")
	}
	_ = slog.Default()
}
