package httptransport

import (
	"errors"
	"net/http"
	"strconv"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.13 transport: the machine JSON contracts of the six handoff
// lifecycle commands (the unconditional human request, the assignment
// round, the assignee's acknowledgement and decline, the client's
// cancellation and the return to Staffy) and the operator PRG forms of
// the «Запрос человека» panel with the assignment card of the assignee.
// The dictionary refusals (the turn anchor, the target roles, the
// deadline word, the result word) are the honest 400 rule words; the
// one-competing-task conflict carries the CURRENT lifecycle words as the
// machine-readable 409 fields; the raced ownership epoch is the honest
// 409 stale signal; the raced mode ladder is the honest 409 conflict
// with the current mode words; the key conflict is the honest 409; a
// foreign tenant is a plain 404; the audit-only member is the 403 with
// the scope words BEFORE every material guard (the M1 precedent of
// 2.5..2.12). The lifecycle changes are announced with aria-live, never
// with a focus move.
// ---------------------------------------------------------------------------

// requestHumanInput is the request of the unconditional ask: the anchor
// turn of the accepted client reply that asked for the human, the CAS
// expectation of the mode ladder version, the target roles of the ask
// (the non-empty subset of the fixed dictionary) and the ack-deadline
// word of the approved policy. The reason of the unconditional ask is
// NEVER a request field — the dictionary constant client_request is the
// system truth (the operator never types it); the OPTIONAL Story 4.3
// boundary word (one of the three operator reasons of the part request
// panel recommendation) is the honest exception — empty stays the
// unconditional ask. The request and event numbers are NEVER request
// fields — the store derives them under the locks.
type requestHumanInput struct {
	TurnRef             *string  `json:"turn_ref"`
	ExpectedModeVersion *int64   `json:"expected_mode_version"`
	TargetRoles         []string `json:"target_roles"`
	AckDeadline         *string  `json:"ack_deadline"`
	Reason              *string  `json:"reason"`
}

// assignHumanHandoffInput is the request of the assignment round: the
// assignee and the backup member references with the round's deadline
// word. The assignee's role word is NEVER a request field — the app
// resolves it from the membership truth (the store re-validates against
// the request's target roles under the locks).
type assignHumanHandoffInput struct {
	Assignee    *string `json:"assignee"`
	Backup      *string `json:"backup"`
	AckDeadline *string `json:"ack_deadline"`
}

// acknowledgeHumanHandoffInput is the request of the assignee's
// confirmed acceptance: the CAS expectation of the ownership epoch (the
// ONE transition that moves the ownership — the composed 2.4 machinery
// of the same transaction).
type acknowledgeHumanHandoffInput struct {
	ExpectedEpoch *int64 `json:"expected_epoch"`
}

// cancelHumanRequestInput is the request of the client's cancellation:
// the anchor turn of the accepted client reply that cancelled the ask.
type cancelHumanRequestInput struct {
	TurnRef *string `json:"turn_ref"`
}

// returnToStaffyInput is the request of the explicit return: the result
// word of the fixed dictionary and the CAS expectation of the ownership
// epoch.
type returnToStaffyInput struct {
	Result        *string `json:"result"`
	ExpectedEpoch *int64  `json:"expected_epoch"`
}

// handoffProblem maps the six command errors onto the problem contract:
// the dictionary and bound refusals are the machine-readable 400 rule
// words; the one-competing-task conflict carries the CURRENT lifecycle
// words (the 409 human_active of the second ask); the raced ownership
// epoch is the honest 409 stale signal; the raced mode ladder is the
// honest 409 conflict with the current mode words; the key conflict is
// the honest 409; a foreign tenant is a plain 404; the audit-only
// member is the 403 with the scope words BEFORE every material guard.
func handoffProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	var active casesDomain.HandoffActiveError
	var conflict casesDomain.ModeConflictError
	switch {
	case errors.As(err, &active):
		server.problem(w, r, http.StatusConflict, "human_active", casesDomain.HandoffActiveText, false, map[string]string{
			"state":          active.State,
			"client_status":  active.ClientStatus,
			"permitted_path": "close_current_request",
		})
	case errors.As(err, &conflict):
		server.problem(w, r, http.StatusConflict, "mode_conflict", casesDomain.ModeConflictText, false, map[string]string{
			"active_mode":    conflict.ActiveMode,
			"reason":         conflict.Reason,
			"signal_source":  conflict.SignalSource,
			"rule_version":   conflict.RuleVersion,
			"mode_version":   strconv.FormatInt(conflict.ModeVersion, 10),
			"permitted_path": "request_human",
		})
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите команду.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение не найдено.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды запроса человека и жизненного цикла передачи требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "handoff command failed", "code", "handoff_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду жизненного цикла передачи. Повторите позднее.", true, nil)
	}
}

// handoffFormMessage renders the honest flash text of the command
// errors.
func handoffFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	var active casesDomain.HandoffActiveError
	switch {
	case errors.As(err, &active):
		return casesDomain.HandoffActiveText
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return "Материал изменился: страница показывает актуальное состояние — сверьте и повторите команду."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение не найдено."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды запроса человека и жизненного цикла передачи требуют полномочия review или configuration."
	}
	return "Команда жизненного цикла передачи не выполнена. Повторите позднее."
}

// handoffFormError flashes the honest message and returns to the case
// page.
func handoffFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	setFlashError(w, tenant, handoffFormMessage(err))
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The machine JSON contracts
// ---------------------------------------------------------------------------

func (s *Server) requestHumanAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input requestHumanInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	turn := stringOrEmpty(input.TurnRef)
	if turn == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref обязательно (UUID): просьба человека якорится принятой репликой клиента.", false, nil)
		return
	}
	turnRef, err := id.Parse(turn)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref обязано быть UUID: просьба человека якорится принятой репликой клиента.", false, nil)
		return
	}
	if input.ExpectedModeVersion == nil || *input.ExpectedModeVersion <= 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле expected_mode_version обязательно (CAS записи режима): перечитайте активный режим и повторите команду.", false, nil)
		return
	}
	roles := input.TargetRoles
	if len(roles) == 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле target_roles обязательно: укажите хотя бы одно слово словаря целевых ролей.", false, nil)
		return
	}
	deadline := stringOrEmpty(input.AckDeadline)
	if deadline == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле ack_deadline обязательно: слово словаря утверждённой policy подтверждения.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.handoffs.RequestHuman(r.Context(), casesAPI.RequestHumanCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		TurnRef: turnRef, ExpectedModeVersion: *input.ExpectedModeVersion,
		TargetRoles: roles, AckDeadline: deadline,
		Reason:         stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		handoffProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) assignHumanHandoffAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input assignHumanHandoffInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	assignee := stringOrEmpty(input.Assignee)
	if assignee == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле assignee обязательно (UUID): назначьте конкретного участника.", false, nil)
		return
	}
	assigneeID, err := id.Parse(assignee)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле assignee обязано быть UUID: назначьте конкретного участника.", false, nil)
		return
	}
	backup := stringOrEmpty(input.Backup)
	if backup == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле backup обязательно (UUID): укажите резервного участника раунда.", false, nil)
		return
	}
	backupID, err := id.Parse(backup)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле backup обязано быть UUID: укажите резервного участника раунда.", false, nil)
		return
	}
	deadline := stringOrEmpty(input.AckDeadline)
	if deadline == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле ack_deadline обязательно: слово словаря утверждённой policy подтверждения.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.handoffs.AssignHumanHandoff(r.Context(), casesAPI.AssignHumanHandoffCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		Assignee: assigneeID, Backup: backupID, AckDeadline: deadline,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		handoffProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) acknowledgeHumanHandoffAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input acknowledgeHumanHandoffInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	if input.ExpectedEpoch == nil || *input.ExpectedEpoch <= 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле expected_epoch обязательно (CAS владения): перечитайте текущего владельца и повторите команду.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.handoffs.AcknowledgeHumanHandoff(r.Context(), casesAPI.AcknowledgeHumanHandoffCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch:  *input.ExpectedEpoch,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		handoffProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) declineHumanHandoffAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.handoffs.DeclineHumanHandoff(r.Context(), casesAPI.DeclineHumanHandoffCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		handoffProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) cancelHumanRequestAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input cancelHumanRequestInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	turn := stringOrEmpty(input.TurnRef)
	if turn == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref обязательно (UUID): отмена якорится принятой репликой клиента.", false, nil)
		return
	}
	turnRef, err := id.Parse(turn)
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref обязано быть UUID: отмена якорится принятой репликой клиента.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.handoffs.CancelHumanRequest(r.Context(), casesAPI.CancelHumanRequestCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		TurnRef:        turnRef,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		handoffProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) returnToStaffyAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input returnToStaffyInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	result := stringOrEmpty(input.Result)
	if result == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле result обязательно: слово словаря результатов возврата.", false, nil)
		return
	}
	if input.ExpectedEpoch == nil || *input.ExpectedEpoch <= 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле expected_epoch обязательно (CAS владения): перечитайте текущего владельца и повторите команду.", false, nil)
		return
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.handoffs.ReturnToStaffy(r.Context(), casesAPI.ReturnToStaffyCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		Result: result, ExpectedEpoch: *input.ExpectedEpoch,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		handoffProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// ---------------------------------------------------------------------------
// The operator forms (PRG)
// ---------------------------------------------------------------------------

// requestHumanForm records the client's unconditional ask: the turn
// select (the accepted client events of the case — the anchor of the
// ask), the target role checkboxes (the fixed dictionary), the deadline
// select (the approved policy dictionary — no silent defaults) and the
// hidden mode version anchor of the rendered form (the deterministic
// idempotency material of the PRG double-submit protection).
func (s *Server) requestHumanForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	turn := r.FormValue("turn_ref")
	if turn == "" {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите реплику клиента: просьба человека якорится принятой репликой."})
		return
	}
	turnRef, err := id.Parse(turn)
	if err != nil {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Реплика клиента обязана быть UUID."})
		return
	}
	roles := r.Form["target_roles"]
	if len(roles) == 0 {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите хотя бы одну целевую роль запроса (словарь фиксирован)."})
		return
	}
	deadline := r.FormValue("ack_deadline")
	if deadline == "" {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите слово дедлайна подтверждения (словарь фиксирован)."})
		return
	}
	expectedVersion, err := strconv.ParseInt(r.FormValue("expected_mode_version"), 10, 64)
	if err != nil || expectedVersion <= 0 {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Текущая версия лестницы обязательна: перечитайте обращение и повторите."})
		return
	}
	anchor := r.FormValue("expected_handoff_requests")
	// The OPTIONAL Story 4.3 boundary reason of the form: the empty
	// choice stays the unconditional client ask (the 2.13 path — the
	// operator never types the system word); one of the three
	// dictionary words rides the existing command (the app guard is
	// fail-closed on the dictionary).
	reason := r.FormValue("reason")
	correlation, _ := correlationFromRequest(r)
	_, err = s.handoffs.RequestHuman(r.Context(), casesAPI.RequestHumanCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		TurnRef: turnRef, ExpectedModeVersion: expectedVersion,
		TargetRoles: roles, AckDeadline: deadline,
		Reason:         reason,
		IdempotencyKey: formCommandKey("request-human", actor.ID.String(), caseID.String(), turn, deadline, strconv.FormatInt(expectedVersion, 10), anchor, joinFormWords(roles), reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		handoffFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "human_requested_recorded")
}

// assignHumanHandoffForm records one assignment round of the open
// lifecycle: the assignee and backup selects (the case's members — the
// honest options of the forms) and the deadline select (the approved
// policy dictionary). The role word of the assignee is NEVER a form
// field — the app resolves it from the membership truth.
func (s *Server) assignHumanHandoffForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	assignee := r.FormValue("assignee")
	if assignee == "" {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите назначаемого специалиста (assignee)."})
		return
	}
	assigneeID, err := id.Parse(assignee)
	if err != nil {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Назначаемый обязан быть UUID."})
		return
	}
	backup := r.FormValue("backup")
	if backup == "" {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите резервного участника раунда (backup)."})
		return
	}
	backupID, err := id.Parse(backup)
	if err != nil {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Резервный участник обязан быть UUID."})
		return
	}
	deadline := r.FormValue("ack_deadline")
	if deadline == "" {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите слово дедлайна подтверждения раунда (словарь фиксирован)."})
		return
	}
	anchor := r.FormValue("expected_handoff_events")
	correlation, _ := correlationFromRequest(r)
	_, err = s.handoffs.AssignHumanHandoff(r.Context(), casesAPI.AssignHumanHandoffCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		Assignee: assigneeID, Backup: backupID, AckDeadline: deadline,
		IdempotencyKey: formCommandKey("assign-handoff", actor.ID.String(), caseID.String(), assignee, backup, deadline, anchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		handoffFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "human_handoff_assigned")
}

// acknowledgeHumanHandoffForm writes the assignee's confirmed
// acceptance: the hidden epoch anchor of the rendered form (the CAS
// expectation of the ownership — the deterministic idempotency material
// of the PRG double-submit protection). The actor IS the assignee (the
// server truth — the guard refuses everyone else).
func (s *Server) acknowledgeHumanHandoffForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expectedEpoch, err := strconv.ParseInt(r.FormValue("expected_epoch"), 10, 64)
	if err != nil || expectedEpoch <= 0 {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Текущая эпоха владения обязательна: перечитайте обращение и повторите."})
		return
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.handoffs.AcknowledgeHumanHandoff(r.Context(), casesAPI.AcknowledgeHumanHandoffCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		ExpectedEpoch:  expectedEpoch,
		IdempotencyKey: formCommandKey("acknowledge-handoff", actor.ID.String(), caseID.String(), strconv.FormatInt(expectedEpoch, 10)),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		handoffFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "human_handoff_acknowledged")
}

// declineHumanHandoffForm writes the assignee's honest refusal of the
// assigned round (the owner never changes — the prior safe owner
// stays).
func (s *Server) declineHumanHandoffForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	anchor := r.FormValue("expected_handoff_events")
	correlation, _ := correlationFromRequest(r)
	_, err := s.handoffs.DeclineHumanHandoff(r.Context(), casesAPI.DeclineHumanHandoffCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		IdempotencyKey: formCommandKey("decline-handoff", actor.ID.String(), caseID.String(), anchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		handoffFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "human_handoff_declined")
}

// cancelHumanRequestForm writes the client's cancellation of the open
// lifecycle: the turn select of the accepted cancelling reply (the
// anchor — the store guard proves the turn belongs to THIS case).
func (s *Server) cancelHumanRequestForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	turn := r.FormValue("turn_ref")
	if turn == "" {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите реплику клиента: отмена якорится принятой репликой."})
		return
	}
	turnRef, err := id.Parse(turn)
	if err != nil {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Реплика клиента обязана быть UUID."})
		return
	}
	anchor := r.FormValue("expected_handoff_events")
	correlation, _ := correlationFromRequest(r)
	_, err = s.handoffs.CancelHumanRequest(r.Context(), casesAPI.CancelHumanRequestCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		TurnRef:        turnRef,
		IdempotencyKey: formCommandKey("cancel-human-request", actor.ID.String(), caseID.String(), turn, anchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		handoffFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "human_request_cancelled")
}

// returnToStaffyForm writes the human owner's explicit return: the
// result select (the fixed dictionary — no silent defaults) and the
// hidden epoch anchor of the rendered form (the CAS expectation of the
// ownership).
func (s *Server) returnToStaffyForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	result := r.FormValue("result")
	if result == "" {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите слово результата возврата (словарь фиксирован)."})
		return
	}
	expectedEpoch, err := strconv.ParseInt(r.FormValue("expected_epoch"), 10, 64)
	if err != nil || expectedEpoch <= 0 {
		handoffFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Текущая эпоха владения обязательна: перечитайте обращение и повторите."})
		return
	}
	anchor := r.FormValue("expected_handoff_events")
	correlation, _ := correlationFromRequest(r)
	_, err = s.handoffs.ReturnToStaffy(r.Context(), casesAPI.ReturnToStaffyCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID,
		Result: result, ExpectedEpoch: expectedEpoch,
		IdempotencyKey: formCommandKey("return-to-staffy", actor.ID.String(), caseID.String(), result, strconv.FormatInt(expectedEpoch, 10), anchor),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		handoffFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "human_handoff_returned")
}

// joinFormWords renders the honest deterministic key material of the
// multi-role form input (the same set of the words — the same key).
func joinFormWords(words []string) string {
	result := ""
	for i, word := range words {
		if i > 0 {
			result += "+"
		}
		result += word
	}
	return result
}
