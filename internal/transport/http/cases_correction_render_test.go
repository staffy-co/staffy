package httptransport

import (
	"bytes"
	"strings"
	"testing"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
)

// The Story 2.15 render smoke tests of the correction surface: the
// markers of the fact rows of the «Контекст передачи» section (the
// read-time derivation of the consistent snapshot — the markers appear
// ONLY with the material), the control branch of the slice (a case
// WITHOUT corrections renders the fact tables byte-for-byte the 2.14
// page) and the «Исправить» form of the fact row of the Work Control
// Card (the PRG form lands on the #decision-context anchor — the section
// position survives, the scroll position does not).

// populatedCorrectionPanel copies the fully populated 2.14 panel and
// attaches the correction state to two fact rows: the applied marker on
// the confirmed row and the disputed marker (with the byte-exact spans
// behind the disclosure) on the conflicting row.
func populatedCorrectionPanel() decisionContextPanelView {
	panel := populatedDecisionContextPanel()
	panel.Facts.Confirmed[0].HasCorrection = true
	panel.Facts.Confirmed[0].MarkerText = "исправлено"
	panel.Facts.Confirmed[0].SourceText = "источник исправления: событие"
	panel.Facts.Confirmed[0].CorrectionNo = "1"
	panel.Facts.Confirmed[0].CorrectionOutcome = "applied"
	panel.Facts.Confirmed[0].CorrectionAtText = "18.10.2026 09:00:00 UTC"
	panel.Facts.Confirmed[0].CorrectionSpansCount = "1 выдержка"
	panel.Facts.Confirmed[0].CorrectionReason = "клиент назвал новый срок"
	panel.Facts.Conflicting[0].HasCorrection = true
	panel.Facts.Conflicting[0].MarkerText = "оспорено (значение системы не изменено)"
	panel.Facts.Conflicting[0].SourceText = "источник исправления: сводка"
	panel.Facts.Conflicting[0].CorrectionNo = "2"
	panel.Facts.Conflicting[0].CorrectionOutcome = "disputed"
	panel.Facts.Conflicting[0].CorrectionAtText = "18.10.2026 09:30:00 UTC"
	panel.Facts.Conflicting[0].CorrectionSpansCount = "2 выдержки"
	panel.Facts.Conflicting[0].CorrectionReason = "значение системы прочитано снова"
	panel.Facts.Conflicting[0].Spans = []string{"цена не совпадает с каталогом", "проверьте актуальную цену"}
	return panel
}

// TestCaseCorrectionMarkerRenders pins the marker row of the fact tables:
// the honest marker with the correction number, the outcome word and the
// time; the source line with the span counter; the byte-exact spans
// behind the progressive disclosure; the follow-up note of the honest
// boundary (the confirmation is the separate command, the dispute never
// rewrites the system value).
func TestCaseCorrectionMarkerRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:          "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID:      "01994f64-6c00-7000-8000-000000000011",
		DecisionContext: populatedCorrectionPanel(),
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the correction markers: %v", err)
	}
	rendered := page.String()
	for _, anchor := range []string{
		"id=\"ctx-correction-01994f64-6c00-7000-8000-00000000ff01\"",
		"исправлено",
		"коррекция №1 (исход applied), 18.10.2026 09:00:00 UTC",
		"Причина: клиент назвал новый срок",
		"источник исправления: событие",
		"оспорено (значение системы не изменено)",
		"коррекция №2 (исход disputed), 18.10.2026 09:30:00 UTC",
		"источник исправления: сводка",
		"Выдержки источника (байт-в-байт)",
		"цена не совпадает с каталогом",
		"проверьте актуальную цену",
		"Подтверждение — отдельная команда точной версии; системное значение спором не перезаписывается",
	} {
		if !strings.Contains(rendered, anchor) {
			t.Errorf("the rendered case page carries the %q anchor of the correction markers", anchor)
		}
	}
}

// TestCaseCorrectionControlBranchRendersByteForByte is the control branch
// of the slice: a case WITHOUT corrections renders the fact tables of the
// «Контекст передачи» section byte-for-byte the 2.14 page — the marker
// {{if}} contributes NOTHING (the markers appear only with the material;
// the fact row emits the exact unchanged bytes).
func TestCaseCorrectionControlBranchRendersByteForByte(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:          "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID:      "01994f64-6c00-7000-8000-000000000011",
		DecisionContext: populatedDecisionContextPanel(),
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render without the corrections: %v", err)
	}
	rendered := page.String()
	// The fact row emits the EXACT unchanged bytes of the 2.14 render: the
	// five cells with the row id, followed by the newline of the range
	// line (the correction {{if}} block contributes nothing).
	confirmedRow := `<tr id="ctx-fact-01994f64-6c00-7000-8000-00000000ff01"><th scope="row">address</th>` +
		`<td>` + casesDomain.ContextFactStatusText(casesDomain.ContextFactConfirmed) + `</td>` +
		`<td>2</td><td>client_event 01994f64-6c00-7000-8000-000000000031 · web_chat</td>` +
		`<td>17.10.2026 10:00:00 UTC</td></tr>` + "\n"
	if !strings.Contains(rendered, confirmedRow) {
		t.Errorf("the fact row without corrections must emit the exact unchanged 2.14 bytes: %q", confirmedRow)
	}
	conflictingRow := `<tr id="ctx-fact-01994f64-6c00-7000-8000-00000000ff03"><th scope="row">service_kind</th>` +
		`<td>` + casesDomain.ContextFactStatusText(casesDomain.ContextFactConflicting) + `</td>` +
		`<td>3</td><td>client_event 01994f64-6c00-7000-8000-000000000033 · web_chat</td>` +
		`<td>17.10.2026 10:10:00 UTC</td></tr>` + "\n"
	if !strings.Contains(rendered, conflictingRow) {
		t.Errorf("the conflicting fact row must emit the exact unchanged 2.14 bytes: %q", conflictingRow)
	}
	// No correction artifacts anywhere: the markers appear only with the
	// material.
	for _, absent := range []string{
		"ctx-correction-",
		"исправлено",
		"оспорено",
		"источник исправления",
		"Выдержки источника",
		"коррекция №",
		"Коррекция записана",
	} {
		if strings.Contains(rendered, absent) {
			t.Errorf("the page without the correction material must NOT carry the %q marker", absent)
		}
	}
}

// TestCaseCorrectionFormRendersOnFactRow pins the PRG form of the fact
// row of the Work Control Card: the version select of the ladder, the
// proposed value, the verbatim span, the summary link, the mandatory
// reason and the honest note of the boundary (the pour is never an
// automatic confirmation, the matrix field disputes without a rewrite).
// The form lives behind the CanManageWork gate of the operator forms
// (the confirm/rescope precedent — the control branch without the gate
// renders nothing).
func TestCaseCorrectionFormRendersOnFactRow(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:        "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID:    "01994f64-6c00-7000-8000-000000000011",
		CanManageWork: true,
		HasWorkItems:  true,
		WorkCards: []workCardView{{
			WorkItemID: "01994f64-6c00-7000-8000-000000000040",
			IndexNo:    "1", TypeText: "Создание", StateText: "В работе", Version: "1", KindText: "create",
			NextAction: "next", CreatedAt: "17.10.2026 09:00:00 UTC",
			HasFacts: true,
			Facts: []factCardView{{
				FactID: "01994f64-6c00-7000-8000-00000000ff01", MaterialKey: "срок", StatusText: "Конфликт значений",
				Versions: []factVersionView{{VersionNo: "1", Value: `{"срок":0}`, SourceType: "client_event", Channel: "web_chat", ObservedAt: "17.10.2026 10:00:00 UTC", ReceivedAt: "17.10.2026 10:00:01 UTC"}},
			}},
		}},
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the correction form: %v", err)
	}
	rendered := page.String()
	formAction := `/app/tenants/01994f64-6c00-7000-8000-000000000002/cases/01994f64-6c00-7000-8000-00000000aa01/facts/01994f64-6c00-7000-8000-00000000ff01/corrections`
	for _, anchor := range []string{
		"Исправить факт срок (операторская коррекция)",
		`action="` + formAction + `"`,
		`name="version_no"`,
		`name="value"`,
		`name="span"`,
		`name="summary_no"`,
		`name="reason"`,
		"Исправить (акт оператора)",
		"заливка applied пишет НОВУЮ версию факта без автоподтверждения",
		"матричные поля (цена/наличие/статус заказа) оспариваются без перезаписи значения системы",
		"Отправка возвращает к контексту передачи (#decision-context)",
	} {
		if !strings.Contains(rendered, anchor) {
			t.Errorf("the rendered work card carries the %q anchor of the correction form", anchor)
		}
	}
	// The gate: without the manage right the form never renders.
	view.CanManageWork = false
	var gated bytes.Buffer
	if err := templates.ExecuteTemplate(&gated, "case", map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}); err != nil {
		t.Fatalf("the gated page does not render: %v", err)
	}
	if strings.Contains(gated.String(), "Исправить факт срок") {
		t.Errorf("the correction form lives behind the CanManageWork gate of the operator forms")
	}
}
