package httptransport

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionPort "github.com/artemiosu/staffy/internal/action/port"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 3.8 (step-03): the recovery-case surfaces — the machine creation
// command of one operation, the machine card/list reads and the Recovery
// Panel pages
// ---------------------------------------------------------------------------
//
// The creation is an EXPLICIT operator command (the drain/reconcile
// precedent — NO daemon): the machine path takes the EMPTY body (the
// strict decode refuses a free field BEFORE the service is ever called),
// the page path takes the deliberate confirmation checkbox with the
// hidden case pin of the PRG return. The eligibility refusals speak the
// honest ladder words (403 — the exact gas, NOTHING is written); the
// idempotent repeat returns the EXISTING case (200 — no second row); the
// evidence append speaks the ONE new append-only row (200). The panel
// pages render the SAFE pins only (AD-10/AC-9 — the material NEVER
// enters), the living receipt chain with the honest unknown words and
// the context-preserving transitions to the original evidence.

// recoveryCreateInput is the strict machine shape of the creation
// request: the body carries NOTHING (the words of the assignment NEVER
// come from a command parameter — the LIVE readings own them; a free
// field is the honest 400 BEFORE the service).
type recoveryCreateInput struct{}

// recoveryCaseRowView is one case of the panel list: the number, the
// trigger word with its fixed text, the assignment words, the deadline
// and the creation moment — the SAFE pins only.
type recoveryCaseRowView struct {
	RecoveryCaseID string
	No             string
	OperationText  string
	Trigger        string
	TriggerText    string
	OwnerText      string
	BackupText     string
	DeadlineText   string
	CreatedAt      string
}

// recoveryEventRowView is one append-only event of the case history.
type recoveryEventRowView struct {
	EventNo string
	Kind    string
	Details string
	At      string
}

// recoveryCaseCardView is the page card of ONE case: the identity pins,
// the trigger and state words with their fixed texts, the assignment,
// the SAFE link pins with the LIVING receipt chain words, the honest
// boundary texts and the append-only history; the transitions carry the
// return pins (the context preservation of AC-12).
type recoveryCaseCardView struct {
	RecoveryCaseID   string
	No               string
	OperationID      string
	CaseID           string
	WorkItemID       string
	DefinitionText   string
	Trigger          string
	TriggerText      string
	State            string
	StateText        string
	OwnerText        string
	BackupText       string
	DeadlineText     string
	CreatedAt        string
	PreviewPinText   string
	ConfirmPinText   string
	ReceiptChainText string
	AttemptsText     string
	ExternalText     string
	OwningSystemText string
	ObservationsText string
	PaymentsText     string
	ProjectionText   string
	Events           []recoveryEventRowView
	HasEvents        bool
}

// recoveryCasesPageView carries the panel page material: the tenant, the
// list, the honest empty state, the flash and the PRG notice.
type recoveryCasesPageView struct {
	TenantID  string
	HasCases  bool
	Cases     []recoveryCaseRowView
	EmptyText string
	Flash     string
	Notice    string
}

// recoveryCasePageView carries the case card page material.
type recoveryCasePageView struct {
	TenantID string
	Card     recoveryCaseCardView
	Policy   recoveryPolicySectionView
	Flash    string
	Notice   string
}

// recoveryPolicyActionRowView is ONE kind row of the «Разрешённые
// действия» section of the case card page: the kind word with the state
// word and the honest reason text visible, the DECLARED attributes in
// the CLOSED details (урок 3-4 — the page speaks the boundary words,
// the detail material rides the disclosure), the confirmation word of
// the material kinds (the AC-13 minimum — «материальный
// recovery-вариант требует отдельного явного подтверждения»
// декларативно).
type recoveryPolicyActionRowView struct {
	Kind        string
	State       string
	StateText   string
	ScopeText   string
	Eligibility string
	Facts       string
	Authority   string
	ConfirmText string
	HasConfirm  bool
}

// recoveryPolicyHumanRowView is ONE human-only decision row of the
// SEPARATE block of the section (AC-1: the class is listed apart from
// the allowed set).
type recoveryPolicyHumanRowView struct {
	Kind         string
	BoundaryText string
}

// recoveryPolicySectionView carries the «Разрешённые действия» section
// of the case card page: the kind rows in the dictionary order, the
// human-only class with the class-level boundary word and the honest
// note of the derivation (read-time вывод — повторный запрос отвечает
// тот же набор, записей нет).
type recoveryPolicySectionView struct {
	Actions     []recoveryPolicyActionRowView
	HumanOnly   []recoveryPolicyHumanRowView
	ClassText   string
	DerivedText string
}

// recoveryCreateStatus answers the honest status of the decision word:
// created — 201 (the case exists NOW); idempotent/evidence — 200 (the
// existing case returns, the repeat is NOT a new resource).
func recoveryCreateStatus(decision string) int {
	if decision == actionPort.RecoveryDecisionCreated {
		return http.StatusCreated
	}
	return http.StatusOK
}

// recoveryCasePageTarget is the PRG target of the creation form: the
// case card page with the decision flag (the notice speaks the honest
// word — created/idempotent/evidence; the focus moves onto it).
func recoveryCasePageTarget(tenantID string, card actionAPI.RecoveryCaseView, decision string) string {
	return "/app/tenants/" + tenantID + "/recovery-cases/" + card.RecoveryCaseID.String() + "?recovery=" + decision
}

// recoveryCasesAPI answers the machine creation command of one operation:
// the strict decode of the EMPTY body, the operator manage gate (the
// words BEFORE any material), the honest decision word with the case
// view — 201 created / 200 idempotent+evidence / 403 the ladder words /
// 404 the unknown operation (the membership blindness).
func (s *Server) recoveryCasesAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.recovery == nil {
		s.notFound(w, r, actor)
		return
	}
	operationID, err := id.Parse(r.PathValue("operation_id"))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "operation_id_invalid", "Идентификатор операции не читается.", false, nil)
		return
	}
	var input recoveryCreateInput
	if err := decodeActionPreviewsJSON(w, r, &input); err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	result, err := s.recovery.CreateRecoveryCase(r.Context(), actionAPI.RecoveryCreateCommand{
		Actor: actor, TenantID: tenant, OperationID: operationID, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, recoveryCreateStatus(result.Decision), map[string]any{"recovery_case": result})
}

// recoveryMachinePolicyAPI answers the machine policy read of one case
// (story 3-9): the READ-TIME derivation of the allowed recovery-action
// set — 200 the view / 401 / 403 the card rights words / 404 the
// membership blindness BEFORE any material (the card precedent). The
// empty GET carries NO body — a free query field is ignored by the
// router, a 400 is NEVER the answer of the empty read.
func (s *Server) recoveryMachinePolicyAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.recovery == nil {
		s.notFound(w, r, actor)
		return
	}
	recoveryCaseID, err := id.Parse(r.PathValue("recovery_case_id"))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "recovery_case_id_invalid", "Идентификатор Recovery Case не читается.", false, nil)
		return
	}
	policy, err := s.recovery.RecoveryCasePolicy(r.Context(), actor, tenant, recoveryCaseID)
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_policy": policy})
}

// newRecoveryPolicySectionView maps the machine policy view onto the
// page section of the case card: the kind rows in the dictionary order
// with the DECLARED attributes in the closed details, the confirmation
// word of the material kinds (the AC-13 minimum), the human-only class
// as the SEPARATE block and the honest derivation note (the words the
// machine view speaks — the page never invents its own copy).
func newRecoveryPolicySectionView(policy actionAPI.RecoveryPolicyView) recoveryPolicySectionView {
	section := recoveryPolicySectionView{
		ClassText:   policy.HumanOnlyClassText,
		DerivedText: "Политика — read-time вывод над записанным: набор выводится заново каждым запросом, записей нет; исполнение видов — будущие срезы через Action Gateway (каждое слово границы права, не обещание результата).",
	}
	for _, action := range policy.Actions {
		row := recoveryPolicyActionRowView{
			Kind:        action.Kind,
			State:       action.State,
			StateText:   action.StateText,
			ScopeText:   action.ScopeText,
			Eligibility: action.EligibilityRuleText,
			Facts:       action.RequiredFactsText,
			Authority:   action.AuthorityText,
			HasConfirm:  action.ConfirmationRequired,
		}
		if action.ConfirmationRequired {
			row.ConfirmText = "Требует отдельного явного подтверждения оператора."
		}
		section.Actions = append(section.Actions, row)
	}
	for _, decision := range policy.HumanOnlyDecisions {
		section.HumanOnly = append(section.HumanOnly, recoveryPolicyHumanRowView{
			Kind:         decision.Kind,
			BoundaryText: decision.BoundaryText,
		})
	}
	return section
}

// recoveryCasesForm answers the page creation form of the eligible
// operation card (the drain/reconcile form precedent): the explicit
// confirmation checkbox (the deliberate operator act), the hidden case
// pin of the PRG return; the refusal words land as the flash, the
// success lands PRG on the case card page with the decision flag.
func (s *Server) recoveryCasesForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.recovery == nil {
		s.notFound(w, r, actor)
		return
	}
	if !validFormOrigin(r) {
		s.operationsProblem(w, r, fmt.Errorf("%w: %s", actionDomain.ErrForbidden, "форма создания Recovery Case доступна только из браузера приложения (origin)."))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		setFlashError(w, tenant, "Проверьте данные формы.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases", http.StatusSeeOther)
		return
	}
	caseID, err := id.Parse(r.FormValue("case_id"))
	if err != nil {
		setFlashError(w, tenant, "Не удалось вернуться на страницу операций: страница кейса не найдена.")
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/cases", http.StatusSeeOther)
		return
	}
	operationID, err := id.Parse(r.FormValue("operation_id"))
	if err != nil {
		setFlashError(w, tenant, "Не удалось создать Recovery Case: операция не читается.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	if r.FormValue("confirm_recovery") != "yes" {
		setFlashError(w, tenant, "Создание Recovery Case требует явного подтверждения: отметьте чекбокс согласия.")
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	result, err := s.recovery.CreateRecoveryCase(r.Context(), actionAPI.RecoveryCreateCommand{
		Actor: actor, TenantID: tenant, OperationID: operationID, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		setFlashError(w, tenant, operationsFormMessage(err))
		http.Redirect(w, r, operationsReviewTarget(tenant, caseID, "x", "1"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, recoveryCasePageTarget(tenant.String(), result.Case, result.Decision), http.StatusSeeOther)
}

// recoveryMachineCardAPI answers the machine card read of one case: the
// SAFE pins with the LIVING receipt chain derived at the read.
func (s *Server) recoveryMachineCardAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.recovery == nil {
		s.notFound(w, r, actor)
		return
	}
	recoveryCaseID, err := id.Parse(r.PathValue("recovery_case_id"))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "recovery_case_id_invalid", "Идентификатор Recovery Case не читается.", false, nil)
		return
	}
	card, err := s.recovery.ReadRecoveryCase(r.Context(), actor, tenant, recoveryCaseID)
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_case": card})
}

// recoveryMachineListAPI answers the machine list read of the tenant's
// cases (bounded by the panel limit).
func (s *Server) recoveryMachineListAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.recovery == nil {
		s.notFound(w, r, actor)
		return
	}
	cases, err := s.recovery.ReadRecoveryCases(r.Context(), actor, tenant)
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_cases": cases})
}

// recoveryNotice speaks the PRG status of the creation form: the decision
// flag carries the honest word (created/idempotent/evidence — the focus
// moves onto the notice).
func recoveryNotice(r *http.Request) string {
	switch raw := r.URL.Query().Get("recovery"); raw {
	case actionPort.RecoveryDecisionCreated:
		return "Recovery Case создан: назначение живыми чтениями, доказательства — SAFE-пины точных версий, исходные записи не переписаны. Фокус переведён на этот статус."
	case actionPort.RecoveryDecisionIdempotent:
		return "Recovery Case уже существует: возвращен существующий кейс — второй строки и события нет (идемпотентность UNIQUE). Фокус переведён на этот статус."
	case actionPort.RecoveryDecisionEvidence:
		return "Новые доказательства добавлены: ОДНА append-only строка evidence_appended с SAFE-дельтой — исходный bundle и квитанции не переписаны. Фокус переведён на этот статус."
	}
	return ""
}

// newRecoveryCaseRowView maps one api case onto the panel row: the SAFE
// pins only (the material NEVER enters).
func newRecoveryCaseRowView(card actionAPI.RecoveryCaseView) recoveryCaseRowView {
	row := recoveryCaseRowView{
		RecoveryCaseID: card.RecoveryCaseID.String(),
		No:             strconv.FormatInt(card.RecoveryCaseNo, 10),
		OperationText:  card.OperationID.String(),
		Trigger:        card.Trigger,
		TriggerText:    card.TriggerText,
		OwnerText:      card.Assignment.OwnerKind + " " + card.Assignment.OwnerRef.String(),
		BackupText:     card.Assignment.BackupOwnerID.String(),
		CreatedAt:      card.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if !card.DeadlineAt.IsZero() {
		row.DeadlineText = card.DeadlineAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return row
}

// recoveryReceiptChainWords speaks the LIVING receipt chain of the card:
// the version count with the latest version and its outcome (AC-10 —
// the card is always fresh; the words are the 3-6/3-7 dictionary).
func recoveryReceiptChainWords(links actionAPI.RecoveryLinksView) string {
	if links.ReceiptVersionsCount == 0 {
		return "Квитанций нет: отправка не выполнялась либо квитанция не записана — слово не выдумывается."
	}
	return "Цепочка квитанций: версий " + strconv.FormatInt(links.ReceiptVersionsCount, 10) +
		", последняя v" + strconv.FormatInt(links.LatestReceiptVersionNo, 10) +
		" — исход " + links.LatestReceiptOutcome + " (живая деривация, актуальность подтверждается сверкой)."
}

// recoveryAttemptsWords speaks the attempt counters of both contours.
func recoveryAttemptsWords(links actionAPI.RecoveryLinksView) string {
	return "Попытки: dispatch " + strconv.Itoa(links.DispatchAttempts) + "/" + strconv.Itoa(actionDomain.MaxDispatchAttempts) +
		", сверка " + strconv.Itoa(links.ReconcileAttempts) + "/" + strconv.Itoa(actionDomain.MaxReconciliationAttempts) +
		" (детали попыток — на карточке операции)."
}

// recoveryExternalWords speaks the external identity of the latest
// receipt (empty — the honest unknown).
func recoveryExternalWords(links actionAPI.RecoveryLinksView) string {
	if links.ExternalObjectID == "" {
		return "Внешний объект не выдан: неизвестное остаётся unknown — слово не выдумывается."
	}
	word := "Внешний объект: " + links.ExternalObjectID
	if links.ExternalObjectVersion != "" {
		word += " (версия " + links.ExternalObjectVersion + ")"
	}
	if links.ExternalObjectTime != "" {
		word += " — " + links.ExternalObjectTime
	}
	return word + ". Локальная проекция — не актуальное внешнее состояние."
}

// recoveryObservationsWords speaks the supporting-observation pins.
func recoveryObservationsWords(links actionAPI.RecoveryLinksView) string {
	if len(links.Observations) == 0 {
		return "Пины наблюдений Owning System: нет (2-7 наблюдения не участвовали)."
	}
	word := "Пины наблюдений Owning System: "
	for i, pin := range links.Observations {
		if i > 0 {
			word += ", "
		}
		word += pin.Ref.String() + ":v" + strconv.FormatInt(pin.Version, 10)
	}
	return word + " (ссылки вместо копий — материал в своих immutable-строках)."
}

// newRecoveryCaseCardView maps the api card onto the page card: the SAFE
// pins with the fixed honest texts and the LIVING chain words.
func newRecoveryCaseCardView(card actionAPI.RecoveryCaseView) recoveryCaseCardView {
	view := recoveryCaseCardView{
		RecoveryCaseID:   card.RecoveryCaseID.String(),
		No:               strconv.FormatInt(card.RecoveryCaseNo, 10),
		OperationID:      card.OperationID.String(),
		CaseID:           card.CaseID.String(),
		WorkItemID:       card.WorkItemID.String(),
		DefinitionText:   card.DefinitionID + "@" + card.DefinitionVersion,
		Trigger:          card.Trigger,
		TriggerText:      card.TriggerText,
		State:            card.State,
		StateText:        card.StateText,
		OwnerText:        card.Assignment.OwnerKind + " " + card.Assignment.OwnerRef.String() + " (эпоха " + strconv.FormatInt(card.Assignment.OwnerEpoch, 10) + ")",
		BackupText:       card.Assignment.BackupOwnerID.String(),
		CreatedAt:        card.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		PreviewPinText:   card.Links.PreviewID.String() + ":" + strconv.FormatInt(card.Links.PreviewVersionNo, 10),
		ConfirmPinText:   card.Links.ConfirmationID.String() + ":" + strconv.FormatInt(card.Links.ConfirmationVersionNo, 10),
		ReceiptChainText: recoveryReceiptChainWords(card.Links),
		AttemptsText:     recoveryAttemptsWords(card.Links),
		ExternalText:     recoveryExternalWords(card.Links),
		OwningSystemText: card.Links.OwningSystem,
		ObservationsText: recoveryObservationsWords(card.Links),
		PaymentsText:     card.PaymentsUnknownText,
		ProjectionText:   card.LocalProjectionText,
	}
	if !card.DeadlineAt.IsZero() {
		view.DeadlineText = card.DeadlineAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	view.HasEvents = len(card.Events) > 0
	for _, event := range card.Events {
		view.Events = append(view.Events, recoveryEventRowView{
			EventNo: strconv.FormatInt(event.EventNo, 10),
			Kind:    event.Kind,
			Details: event.Details,
			At:      event.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return view
}

// recoveryCasesPage answers the Recovery Panel page: the tenant's cases
// ordered by the creation moment descending (bounded), the honest empty
// state, the flash and the PRG notice.
func (s *Server) recoveryCasesPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.recovery == nil {
		s.notFound(w, r, actor)
		return
	}
	cases, err := s.recovery.ReadRecoveryCases(r.Context(), actor, tenant)
	if err != nil {
		s.operationsProblem(w, r, err)
		return
	}
	page := recoveryCasesPageView{
		TenantID:  tenant.String(),
		EmptyText: "Recovery Case пока нет: кейсы восстановления создаются командой оператора из карточки eligible-операции (автоматика исчерпана либо недоступна).",
		Flash:     flashError(r, tenant),
		Notice:    recoveryNotice(r),
	}
	for _, card := range cases {
		page.Cases = append(page.Cases, newRecoveryCaseRowView(card))
	}
	page.HasCases = len(page.Cases) > 0
	s.render(w, "recovery_cases", page)
}

// recoveryCasePage answers the case card page: the SAFE pins with the
// LIVING chain words, the assignment, the append-only history and the
// context-preserving transitions to the original evidence (the case
// card, the operation card — the reconcile launch rides the operation
// surface of 3-7, NO new query path exists).
func (s *Server) recoveryCasePage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.casesActor(w, r)
	if !ok {
		return
	}
	if s.recovery == nil {
		s.notFound(w, r, actor)
		return
	}
	recoveryCaseID, err := id.Parse(r.PathValue("recovery_case_id"))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "recovery_case_id_invalid", "Идентификатор Recovery Case не читается.", false, nil)
		return
	}
	card, err := s.recovery.ReadRecoveryCase(r.Context(), actor, tenant, recoveryCaseID)
	if err != nil {
		if errors.Is(err, actionPort.ErrRecoveryNotFound) {
			s.notFound(w, r, actor)
			return
		}
		s.operationsProblem(w, r, err)
		return
	}
	// The «Разрешённые действия» section of the card (story 3-9): the
	// READ-TIME policy derivation rides the SAME card rights (the
	// 403/404 words of the card surface; the section renders honestly
	// empty on a transient derivation miss — the card itself never
	// refuses).
	section := recoveryPolicySectionView{}
	if policy, policyErr := s.recovery.RecoveryCasePolicy(r.Context(), actor, tenant, recoveryCaseID); policyErr == nil {
		section = newRecoveryPolicySectionView(policy)
	}
	page := recoveryCasePageView{
		TenantID: tenant.String(),
		Card:     newRecoveryCaseCardView(card),
		Policy:   section,
		Flash:    flashError(r, tenant),
		Notice:   recoveryNotice(r),
	}
	s.render(w, "recovery_case", page)
}

// BindRecovery registers the recovery-case surface of story 3.8: the
// machine creation command of one operation, the machine card/list reads
// and the Recovery Panel pages. The service is the narrow
// api.RecoveryService (the BindReconcile type-assert precedent — the app
// Service carries the compile-time proof); the production root binds the
// REAL service — the contour is LOCAL (no adapter exists anywhere, AG-4
// does NOT refuse this one: the recovery cases get created honestly).
func (s *Server) BindRecovery(service actionAPI.RecoveryService) {
	s.recovery = service
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/operations/{operation_id}/recovery-case", s.recoveryCasesAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/recovery-cases", s.recoveryMachineListAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/recovery-cases/{recovery_case_id}", s.recoveryMachineCardAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/recovery-cases/{recovery_case_id}/policy", s.recoveryMachinePolicyAPI)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/operations/{operation_id}/recovery-case", s.recoveryCasesForm)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/recovery-cases", s.recoveryCasesPage)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/recovery-cases/{recovery_case_id}", s.recoveryCasePage)
}
