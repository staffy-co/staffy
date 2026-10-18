package httptransport

import (
	"bytes"
	"strings"
	"testing"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
)

// The Story 2.16 render smoke tests of the waiting update surface: the
// «Ожидание человека» block of the last round (the honest client waiting
// status in the three distinctions of AC-1 with the promise line and the
// missed next step — the block appears ONLY with the promise material),
// the control branch of the slice (a round WITHOUT promises renders the
// «Запрос человека» section lines byte-for-byte the 2.13 page — the
// waiting {{if}} contributes NOTHING) and the «Обещать обновление» form
// of the handoff section (the PRG form lands on the #handoff anchor).

// populatedWaitingPanel builds the waiting block of the last round with
// the promise material: the assigned waiting status, the last promise of
// the 1h word with the computed promised time and the missed state with
// the honest next step.
func populatedWaitingPanel(missed bool) waitingUpdatePanelView {
	state := casesDomain.UpdatePromisePending
	if missed {
		state = casesDomain.UpdatePromiseMissed
	}
	view := waitingUpdatePanelView{
		HasPromise:       true,
		ClientStatus:     casesDomain.ClientWaitingAssigned,
		ClientStatusText: casesDomain.ClientWaitingStatusText(casesDomain.ClientWaitingAssigned),
		PromiseNo:        "2",
		DeadlineWord:     casesDomain.UpdateDeadline1h,
		DeadlineText:     casesDomain.UpdateDeadlineText(casesDomain.UpdateDeadline1h),
		HasDeadline:      true,
		PromisedAtText:   "18.10.2026 13:00:00 UTC",
		State:            state,
		StateText:        casesDomain.UpdatePromiseStateText(state),
		PromiseLineText:  "Обновление обещано: в течение часа. Ожидаем обновления к обещанному сроку.",
		PromiseAnchor:    "2",
	}
	if missed {
		view.PromiseLineText = "Обновление обещано: в течение часа. Обновление просрочено — решения ещё нет. Следующий шаг: сообщите статус существующим ответом клиенту."
		view.MissedNextStepText = casesDomain.UpdatePromiseMissedNextStepText
	}
	return view
}

// promiseDeadlineOptionsOf builds the honest form options of the fixed
// deadline dictionary (the render material of the form select).
func promiseDeadlineOptionsOf() []workOption {
	options := []workOption{}
	for _, word := range casesDomain.UpdateDeadlines {
		options = append(options, workOption{Value: word, Text: casesDomain.UpdateDeadlineText(word)})
	}
	return options
}

// TestCaseWaitingBlockRenders pins the «Ожидание человека» block: the
// honest waiting status line of the last round (the dictionary word with
// the text — three distinctions of AC-1), the promise line with the
// number, the deadline word text, the computed promised time and the
// derived state, the client line composition and the honest missed next
// step of the overdue promise (the update is the EXTERNAL producer — the
// human sends it by the existing reply path).
func TestCaseWaitingBlockRenders(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:     "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID: "01994f64-6c00-7000-8000-000000000011",
		Waiting:    populatedWaitingPanel(false),
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the waiting block: %v", err)
	}
	rendered := page.String()
	for _, anchor := range []string{
		`id="human-waiting"`,
		`id="waiting-status"`,
		"Статус ожидания: <strong>" + casesDomain.ClientWaitingStatusText(casesDomain.ClientWaitingAssigned) + "</strong>",
		`(слово: ` + casesDomain.ClientWaitingAssigned + `)`,
		`id="waiting-promise"`,
		"Обещание обновления №2 — " + casesDomain.UpdateDeadlineText(casesDomain.UpdateDeadline1h),
		"Обещанный срок: 18.10.2026 13:00:00 UTC",
		`(слово: ` + casesDomain.UpdatePromisePending + `)`,
		`id="waiting-client-line"`,
		"Клиенту: Обновление обещано: в течение часа. Ожидаем обновления к обещанному сроку.",
	} {
		if !strings.Contains(rendered, anchor) {
			t.Errorf("the rendered case page carries the %q anchor of the waiting block", anchor)
		}
	}
	// The missed promise carries the honest overdue word with the next
	// step (the anti-disclosure: the block carries NO persona — the
	// operator names, the epochs and the identifiers are absent from
	// every client line).
	missedView := view
	missedView.Waiting = populatedWaitingPanel(true)
	var missed bytes.Buffer
	if err := templates.ExecuteTemplate(&missed, "case", map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": missedView}); err != nil {
		t.Fatalf("the case page does not render with the missed promise: %v", err)
	}
	missedRendered := missed.String()
	for _, anchor := range []string{
		`id="waiting-missed"`,
		"Обновление просрочено — решения ещё нет.",
		casesDomain.UpdatePromiseMissedNextStepText,
		`(слово: ` + casesDomain.UpdatePromiseMissed + `)`,
	} {
		if !strings.Contains(missedRendered, anchor) {
			t.Errorf("the rendered case page carries the %q anchor of the missed promise", anchor)
		}
	}
	// The PRG success line rides the URL flag (the honest status line of
	// the section — the flag of the «Обещать обновление» form).
	successView := view
	successView.UpdatePromised = true
	var success bytes.Buffer
	if err := templates.ExecuteTemplate(&success, "case", map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": successView}); err != nil {
		t.Fatalf("the case page does not render with the PRG flag: %v", err)
	}
	if !strings.Contains(success.String(), `id="waiting-promised"`) || !strings.Contains(success.String(), "Обещание обновления записано (append-only)") {
		t.Errorf("the PRG success line renders with the honest status words of the section")
	}
}

// TestCaseWaitingControlBranchRendersByteForByte is the control branch
// of the slice: a round WITHOUT promises renders the «Запрос человека»
// section lines byte-for-byte the 2.13 page — the waiting {{if}} block
// contributes NOTHING (the block appears only with the promise
// material; the current state line emits the exact unchanged bytes).
func TestCaseWaitingControlBranchRendersByteForByte(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:             "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID:         "01994f64-6c00-7000-8000-000000000011",
		HandoffsPanelText:  "Запрос человека — запись жизненного цикла обращения.",
		HandoffCurrentText: "Запрос человека: Назначен: ожидание подтверждения специалистом. (слово: assigned, запрос №1)",
		HandoffClientText:  "Клиентский статус: Специалист назначен и ещё не подтвердил принятие обращения.",
		HasHandoffs:        true,
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render without the promises: %v", err)
	}
	rendered := page.String()
	// The current state line of the section emits the EXACT unchanged
	// bytes of the 2.13 render (the waiting {{if}} block contributes
	// nothing — the line is followed directly by the lifecycle card
	// area).
	currentLine := `<p id="handoffs-current" aria-live="polite"><strong>Запрос человека: Назначен: ожидание подтверждения специалистом. (слово: assigned, запрос №1)</strong> Клиентский статус: Специалист назначен и ещё не подтвердил принятие обращения.</p>` + "\n"
	if !strings.Contains(rendered, currentLine) {
		t.Errorf("the current state line without promises must emit the exact unchanged 2.13 bytes: %q", currentLine)
	}
	// No waiting artifacts anywhere: the block appears only with the
	// promise material (the PRG success line rides the URL flag — never
	// the material).
	for _, absent := range []string{
		`id="human-waiting"`,
		`id="waiting-status"`,
		`id="waiting-promise"`,
		`id="waiting-client-line"`,
		`id="waiting-missed"`,
		`id="waiting-promised"`,
		"Ожидание человека — честное слово ожидания",
		"Обещание обновления №",
		"Клиенту: Обновление обещано",
	} {
		if strings.Contains(rendered, absent) {
			t.Errorf("the page without the promise material must NOT carry the %q marker", absent)
		}
	}
}

// TestCaseWaitingFormRendersInHandoffSection pins the PRG form
// «Обещать обновление» of the handoff section: the deadline select of
// the fixed dictionary (the honest unknown word included — no silent
// defaults), the hidden promise anchor of the double-submit protection
// and the honest note of the boundary (the promise never replaces the
// reply, the update is sent by the human by the existing reply path).
// The form lives behind the CanManageWork gate of the operator forms
// (the assign/cancel precedent — the control branch without the gate
// renders nothing).
func TestCaseWaitingFormRendersInHandoffSection(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	view := caseDetailPageView{
		CaseID:                 "01994f64-6c00-7000-8000-00000000aa01",
		EmployeeID:             "01994f64-6c00-7000-8000-000000000011",
		CanManageWork:          true,
		HasHandoffs:            true,
		PromiseDeadlineOptions: promiseDeadlineOptionsOf(),
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the promise form: %v", err)
	}
	rendered := page.String()
	formAction := `/app/tenants/01994f64-6c00-7000-8000-000000000002/cases/01994f64-6c00-7000-8000-00000000aa01/update-promises`
	for _, anchor := range []string{
		"Обещать обновление (promise_handoff_update — честный срок или честное неизвестное)",
		`action="` + formAction + `"`,
		`name="deadline_word"`,
		`name="expected_update_promises"`,
		"Обновление обещано: в течение 15 минут.",
		"Обновление обещано: в течение часа.",
		"Обновление обещано: в течение четырёх часов.",
		casesDomain.UpdateDeadlineText(casesDomain.UpdateDeadlineUnknown),
		"Обещать обновление",
		"обещание НЕ заменяет ответ и НЕ меняет владельца — обновление отправляется существующим ответом клиенту",
		"просрочка — честное слово read-time, без автоматических таймеров и эскалации",
	} {
		if !strings.Contains(rendered, anchor) {
			t.Errorf("the rendered handoff section carries the %q anchor of the promise form", anchor)
		}
	}
	// The gate: without the manage right the form never renders.
	view.CanManageWork = false
	var gated bytes.Buffer
	if err := templates.ExecuteTemplate(&gated, "case", map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002", "View": view}); err != nil {
		t.Fatalf("the gated page does not render: %v", err)
	}
	if strings.Contains(gated.String(), "Обещать обновление (promise_handoff_update") {
		t.Errorf("the promise form lives behind the CanManageWork gate of the operator forms")
	}
}
