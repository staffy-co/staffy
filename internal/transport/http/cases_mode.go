package httptransport

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.5 transport: the machine JSON contract of the four mode
// commands and the operator forms (PRG) of the Conversation Mode panel
// and the disclosure surface. The mode gate and the disclosure gate
// refusals of the client reply are machine-readable (the problem fields
// carry the effective mode / the channel and the permitted path — the UI
// shows them directly). The dynamic mode change is announced with
// aria-live, never with a focus move.
// ---------------------------------------------------------------------------

type proposeModeInput struct {
	ConversationID      *string `json:"conversation_id"`
	Mode                *string `json:"mode"`
	Scope               *string `json:"scope"`
	WorkItemID          *string `json:"work_item_id"`
	ExpectedModeVersion *int64  `json:"expected_mode_version"`
	Reason              *string `json:"reason"`
	SignalSource        *string `json:"signal_source"`
	TurnRef             *string `json:"turn_ref"`
}

type exitModeInput struct {
	ConversationID      *string `json:"conversation_id"`
	Mode                *string `json:"mode"`
	ExpectedModeVersion *int64  `json:"expected_mode_version"`
	Reason              *string `json:"reason"`
}

type recordDisclosureInput struct {
	ConversationID *string `json:"conversation_id"`
	Channel        *string `json:"channel"`
	Text           *string `json:"text"`
	ParentRef      *string `json:"parent_ref"`
}

type recordDisclosureEventInput struct {
	ConversationID *string `json:"conversation_id"`
	Kind           *string `json:"kind"`
}

// modeProblem maps the mode command errors onto the problem contract: the
// machine-readable gate refusals carry their words as the problem fields;
// the raced transitions are the honest 409 with the current mode; the
// rule refusals are 400; a foreign tenant is a plain 404.
func modeProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	var conflict casesDomain.ModeConflictError
	var modeGate casesDomain.ReplyModeError
	var disclosureGate casesDomain.ReplyDisclosureError
	switch {
	case errors.As(err, &conflict):
		server.problem(w, r, http.StatusConflict, "mode_conflict", casesDomain.ModeConflictText, false, map[string]string{
			"active_mode":    conflict.ActiveMode,
			"reason":         conflict.Reason,
			"signal_source":  conflict.SignalSource,
			"rule_version":   conflict.RuleVersion,
			"mode_version":   strconv.FormatInt(conflict.ModeVersion, 10),
			"permitted_path": "propose_mode",
		})
	case errors.As(err, &modeGate):
		server.problem(w, r, http.StatusForbidden, "reply_mode_gate", modeGate.Reason, false, map[string]string{
			"effective_mode": modeGate.EffectiveMode,
			"permitted_path": casesDomain.PermittedPathExit,
		})
	case errors.As(err, &disclosureGate):
		server.problem(w, r, http.StatusForbidden, "reply_disclosure_gate", disclosureGate.Reason, false, map[string]string{
			"channel":        disclosureGate.Channel,
			"permitted_path": casesDomain.PermittedPathRecordDisclosure,
		})
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", casesDomain.ModeConflictText, false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или материал режима не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды режима требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "mode command failed", "code", "mode_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду. Повторите позднее.", true, nil)
	}
}

// modeFormError flashes the honest message and returns to the case page
// (the machine-readable words travel inside the text — the form path
// shows them directly).
func modeFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	message := modeFormMessage(err)
	setFlashError(w, tenant, message)
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// modeFormMessage renders the honest flash text of the mode command
// errors (the shared shape of the ownership form errors).
func modeFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	var conflict casesDomain.ModeConflictError
	var modeGate casesDomain.ReplyModeError
	var disclosureGate casesDomain.ReplyDisclosureError
	switch {
	case errors.As(err, &conflict):
		return casesDomain.ModeConflictText + " Текущий режим: " + conflict.ActiveMode + " (версия " + strconv.FormatInt(conflict.ModeVersion, 10) + ")."
	case errors.As(err, &modeGate):
		return modeGate.Reason
	case errors.As(err, &disclosureGate):
		return disclosureGate.Reason
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return casesDomain.ModeConflictText
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение или материал режима не найдены."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды режима требуют полномочия review или configuration."
	}
	return "Действие не выполнено. Повторите позднее."
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

func (s *Server) proposeModeAPI(w http.ResponseWriter, r *http.Request) {
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
	var input proposeModeInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil || !conversationID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id обязательно (UUID): режим управляется на диалоге обращения.", false, nil)
		return
	}
	workItemID := id.UUID("")
	if raw := stringOrEmpty(input.WorkItemID); raw != "" {
		if workItemID, err = id.Parse(raw); err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле work_item_id должно быть UUID.", false, nil)
			return
		}
	}
	turnRef := id.UUID("")
	if raw := stringOrEmpty(input.TurnRef); raw != "" {
		if turnRef, err = id.Parse(raw); err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref должно быть UUID.", false, nil)
			return
		}
	}
	if input.ExpectedModeVersion == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле expected_mode_version обязательно: сверьте текущую версию лестницы режимов.", false, nil)
		return
	}
	outcome, err := s.cases.ProposeMode(r.Context(), casesAPI.ProposeModeCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: stringOrEmpty(input.Mode), Scope: stringOrEmpty(input.Scope), WorkItemID: workItemID,
		ExpectedModeVersion: *input.ExpectedModeVersion, Reason: stringOrEmpty(input.Reason),
		SignalSource: stringOrEmpty(input.SignalSource), TurnRef: turnRef,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		modeProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) exitModeAPI(w http.ResponseWriter, r *http.Request) {
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
	var input exitModeInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil || !conversationID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id обязательно (UUID): режим управляется на диалоге обращения.", false, nil)
		return
	}
	if input.ExpectedModeVersion == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле expected_mode_version обязательно: сверьте текущую версию лестницы режимов.", false, nil)
		return
	}
	outcome, err := s.cases.ExitMode(r.Context(), casesAPI.ExitModeCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: stringOrEmpty(input.Mode), ExpectedModeVersion: *input.ExpectedModeVersion,
		Reason: stringOrEmpty(input.Reason), IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		modeProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) recordDisclosureAPI(w http.ResponseWriter, r *http.Request) {
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
	var input recordDisclosureInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil || !conversationID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id обязательно (UUID): раскрытие живёт на диалоге обращения.", false, nil)
		return
	}
	parentRef := id.UUID("")
	if raw := stringOrEmpty(input.ParentRef); raw != "" {
		if parentRef, err = id.Parse(raw); err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле parent_ref должно быть UUID.", false, nil)
			return
		}
	}
	outcome, err := s.cases.RecordDisclosure(r.Context(), casesAPI.RecordDisclosureCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: stringOrEmpty(input.Channel), Text: stringOrEmpty(input.Text), ParentRef: parentRef,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		modeProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) recordDisclosureEventAPI(w http.ResponseWriter, r *http.Request) {
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
	disclosureID, err := id.Parse(r.PathValue("disclosure_id"))
	if err != nil || !disclosureID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Идентификатор раскрытия должен быть UUID.", false, nil)
		return
	}
	var input recordDisclosureEventInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil || !conversationID.Valid() {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id обязательно (UUID): раскрытие живёт на диалоге обращения.", false, nil)
		return
	}
	outcome, err := s.cases.RecordDisclosureEvent(r.Context(), casesAPI.RecordDisclosureEventCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosureID, Kind: stringOrEmpty(input.Kind),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		modeProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// ---------------------------------------------------------------------------
// The operator forms (PRG)
// ---------------------------------------------------------------------------

func (s *Server) proposeModeForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог обращения."})
		return
	}
	expected, err := formInt64(r, "expected_mode_version")
	if err != nil {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию лестницы режимов."})
		return
	}
	mode := r.FormValue("mode")
	scope := r.FormValue("scope")
	reason := r.FormValue("reason")
	signalSource := r.FormValue("signal_source")
	turnRef := id.UUID("")
	if signalSource == casesDomain.ModeSignalTurnEvent {
		turnRef, err = formUUID(r, "turn_ref")
		if err != nil {
			modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сигнал turn_event обязан ссылаться на событие кейса."})
			return
		}
	}
	workItemID := id.UUID("")
	if scope == casesDomain.ModeScopeWorkItem {
		workItemID, err = formUUID(r, "work_item_id")
		if err != nil {
			modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Контекст режима требует элемент работы."})
			return
		}
	}
	_, err = s.cases.ProposeMode(r.Context(), casesAPI.ProposeModeCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: mode, Scope: scope, WorkItemID: workItemID, ExpectedModeVersion: expected,
		Reason: reason, SignalSource: signalSource, TurnRef: turnRef,
		IdempotencyKey: formCommandKey("propose-mode", actor.ID.String(), caseID.String(), conversationID.String(), strconv.FormatInt(expected, 10), mode, scope, workItemID.String(), reason, signalSource, turnRef.String()),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		modeFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "mode_proposed")
}

func (s *Server) exitModeForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог обращения."})
		return
	}
	expected, err := formInt64(r, "expected_mode_version")
	if err != nil {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Сверьте текущую версию лестницы режимов."})
		return
	}
	mode := r.FormValue("mode")
	reason := r.FormValue("reason")
	_, err = s.cases.ExitMode(r.Context(), casesAPI.ExitModeCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Mode: mode, ExpectedModeVersion: expected, Reason: reason,
		IdempotencyKey: formCommandKey("exit-mode", actor.ID.String(), caseID.String(), conversationID.String(), strconv.FormatInt(expected, 10), mode, reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		modeFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "mode_exited")
}

func (s *Server) recordDisclosureForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог обращения."})
		return
	}
	channel := r.FormValue("channel")
	text := r.FormValue("text")
	parentRef := id.UUID("")
	if raw := r.FormValue("parent_ref"); raw != "" {
		if parentRef, err = id.Parse(raw); err != nil {
			modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Родительская запись должна быть UUID."})
			return
		}
	}
	_, err = s.cases.RecordDisclosure(r.Context(), casesAPI.RecordDisclosureCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: channel, Text: text, ParentRef: parentRef,
		IdempotencyKey: formCommandKey("record-disclosure", actor.ID.String(), caseID.String(), conversationID.String(), channel, text, parentRef.String()),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		modeFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "disclosure_recorded")
}

func (s *Server) recordDisclosureEventForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	disclosureID, err := id.Parse(r.PathValue("disclosure_id"))
	if err != nil || !disclosureID.Valid() {
		s.notFound(w, r, actor)
		return
	}
	if !formParse(w, r) {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		modeFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог обращения."})
		return
	}
	kind := r.FormValue("kind")
	_, err = s.cases.RecordDisclosureEvent(r.Context(), casesAPI.RecordDisclosureEventCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: disclosureID, Kind: kind,
		IdempotencyKey: formCommandKey("record-disclosure-event", actor.ID.String(), caseID.String(), conversationID.String(), disclosureID.String(), kind),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		modeFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "disclosure_event_recorded")
}

// sendClientReplyModeForm is the Story 2.5 extension of the reply form:
// the reply class select (required — no silent default) and the optional
// Work Item anchor select. The command itself stays the 2-4 one with the
// class and the anchor material.
func (s *Server) sendClientReplyClass(r *http.Request) (string, id.UUID, error) {
	class := r.FormValue("reply_class")
	if err := casesDomain.GuardReplyClass(class); err != nil {
		return "", id.UUID(""), casesDomain.WorkRuleError{Rule: "send_client_reply", Reason: casesDomain.ReplyClassRequiredText}
	}
	workItemID := id.UUID("")
	if raw := r.FormValue("work_item_id"); raw != "" && !strings.HasPrefix(raw, "—") {
		parsed, err := id.Parse(raw)
		if err != nil {
			return "", id.UUID(""), casesDomain.WorkRuleError{Rule: "send_client_reply", Reason: "Выберите элемент работы ответа."}
		}
		workItemID = parsed
	}
	return class, workItemID, nil
}
