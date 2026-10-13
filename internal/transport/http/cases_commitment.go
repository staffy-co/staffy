package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 2.9 transport: the machine JSON contract of the three commitment
// commands, the operator PRG forms of the «Обязательства и подтверждения»
// panel and the Commitment Review page. The dictionary refusals (the status,
// the terms, the actions, the unknown holes) are the honest 400 rule words;
// the span outside the payload is the honest 400 span-not-in-source; the
// missing presentation link is the honest 400 not-presented; the stale or
// conflicting observation is the honest 400 freshness; the insufficient
// assurance is the honest 400 with the step-up ladder words; the duplicate
// subject of the same version is the honest 409; the key conflict is the
// honest 409; a foreign tenant is a plain 404; the audit-only member is the
// 403 with the scope words BEFORE any material guard. The version changes
// are announced with aria-live text and a focus transfer to the description
// (AC-12); the confirm button is never focused nor preselected.
// ---------------------------------------------------------------------------

// splitCommitmentLink splits the "<uuid>:<version>" option value of the
// presentation link select of the reply form.
func splitCommitmentLink(value string) (id.UUID, int64, error) {
	return formOption(value)
}

// commitmentProblem maps the commitment command errors onto the problem
// contract: the dictionary and discipline refusals are the machine-readable
// 400 rule words; the duplicate subject is the honest 409; the key conflict
// and the stale ladder are the honest 409; a foreign tenant is a plain 404;
// the audit-only member is the 403 with the scope words.
func commitmentProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrCommitmentDuplicate):
		server.problem(w, r, http.StatusConflict, "commitment_duplicate", "Подтверждение этого субъекта уже записано для точной версии: одно подтверждение субъекта на версию.", false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущее состояние и повторите команду.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение или элемент работы не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Команды обязательств требуют полномочия review или configuration.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "commitment command failed", "code", "commitment_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду обязательства. Повторите позднее.", true, nil)
	}
}

// commitmentFormMessage renders the honest flash text of the command errors.
func commitmentFormMessage(err error) string {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		return rule.Reason
	case errors.Is(err, casesDomain.ErrCommitmentDuplicate):
		return "Подтверждение этого субъекта уже записано для точной версии: одно подтверждение субъекта на версию."
	case errors.Is(err, casesDomain.ErrStaleVersion):
		return "Материал изменился (лестница версий или элемент работы): сверьте текущее состояние и повторите команду."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		return "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		return "Обращение или элемент работы не найдены."
	case errors.Is(err, domain.ErrForbidden):
		return "Команды обязательств требуют полномочия review или configuration."
	}
	return "Команда обязательства не выполнена. Повторите позднее."
}

// commitmentFormError flashes the honest message and returns to the
// Commitment Review page.
func commitmentFormError(w http.ResponseWriter, r *http.Request, tenant, caseID, workItemID id.UUID, err error) {
	setFlashError(w, tenant, commitmentFormMessage(err))
	http.Redirect(w, r, commitmentReviewTarget(tenant, caseID, workItemID), http.StatusSeeOther)
}

// commitmentReviewTarget is the PRG target of the commitment forms — the
// Commitment Review page of one Work Item.
func commitmentReviewTarget(tenant, caseID, workItemID id.UUID) string {
	return "/app/tenants/" + tenant.String() + "/cases/" + caseID.String() + "/work-items/" + workItemID.String() + "/commitments"
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

type createCommitmentInput struct {
	CommitmentID     *string                    `json:"commitment_id"`
	Status           *string                    `json:"status"`
	Terms            *casesDomain.MaterialTerms `json:"terms"`
	ActionSet        *[]string                  `json:"action_set"`
	ValidUntil       *string                    `json:"valid_until"`
	ConsumptionLimit *int                       `json:"consumption_limit"`
	MaterialUnknowns *[]string                  `json:"material_unknowns"`
}

func (s *Server) createCommitmentAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input createCommitmentInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	status := stringOrEmpty(input.Status)
	if status == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле status обязательно: выберите слово словаря (draft, offered).", false, nil)
		return
	}
	if input.Terms == nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле terms обязательно: материальные условия версии.", false, nil)
		return
	}
	actions := []string{}
	if input.ActionSet != nil {
		actions = *input.ActionSet
	}
	unknowns := []string{}
	if input.MaterialUnknowns != nil {
		unknowns = *input.MaterialUnknowns
	}
	limit := 1
	if input.ConsumptionLimit != nil {
		limit = *input.ConsumptionLimit
	}
	validUntil, err := parseTimeLoose(stringOrEmpty(input.ValidUntil))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле valid_until обязательно (RFC 3339, будущее).", false, nil)
		return
	}
	commitmentID := id.UUID("")
	if input.CommitmentID != nil && *input.CommitmentID != "" {
		if commitmentID, err = id.Parse(*input.CommitmentID); err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле commitment_id должно быть UUID.", false, nil)
			return
		}
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.cases.CreateCommitment(r.Context(), casesAPI.CreateCommitmentCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, Status: status, Terms: *input.Terms, ActionSet: actions,
		ValidUntil: validUntil, ConsumptionLimit: limit, MaterialUnknowns: unknowns,
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		commitmentProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

// parseTimeLoose parses the RFC 3339 time of the command inputs.
func parseTimeLoose(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("пустое время")
	}
	return time.Parse(time.RFC3339, value)
}

// parseFormTimeLoose parses the valid-until of the create FORM: the
// datetime-local input of the browser submits «YYYY-MM-DDTHH:MM» (or with
// seconds) without the zone word — the honest operator surface accepts it
// as UTC (the fixture clock and the server truth are UTC); the explicit
// RFC 3339 form with a zone stays the machine path of the API.
func parseFormTimeLoose(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("пустое время")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, errors.New("время не разобрано")
}

// formInt64Value parses the optional integer form field (empty — the
// honest zero).
func formInt64Value(r *http.Request, field string) (int64, error) {
	value := strings.TrimSpace(r.FormValue(field))
	if value == "" {
		return 0, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

type confirmCommitmentInput struct {
	VersionNo       *int64    `json:"version_no"`
	ExpectedVersion *int64    `json:"expected_version"`
	SubjectKind     *string   `json:"subject_kind"`
	SubjectRef      *string   `json:"subject_ref"`
	TurnRef         *string   `json:"turn_ref"`
	Spans           *[]string `json:"spans"`
}

func (s *Server) confirmCommitmentAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	commitmentID, err := id.Parse(r.PathValue("commitment_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input confirmCommitmentInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	if input.VersionNo == nil || *input.VersionNo < 1 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле version_no обязательно (>= 1): точная показанная версия.", false, nil)
		return
	}
	if input.ExpectedVersion == nil || *input.ExpectedVersion < 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле expected_version обязательно: ожидаемая версия элемента работы (CAS).", false, nil)
		return
	}
	subjectKind := "client"
	if input.SubjectKind != nil {
		subjectKind = *input.SubjectKind
	}
	subjectRef := stringOrEmpty(input.SubjectRef)
	if subjectRef == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле subject_ref обязательно: подтверждаемый субъект.", false, nil)
		return
	}
	turnRef, err := id.Parse(stringOrEmpty(input.TurnRef))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref должно быть UUID принятого текстового события кейса.", false, nil)
		return
	}
	spans := []string{}
	if input.Spans != nil {
		spans = *input.Spans
	}
	correlation, _ := correlationFromRequest(r)
	outcome, err := s.cases.ConfirmCommitment(r.Context(), casesAPI.ConfirmCommitmentCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, VersionNo: *input.VersionNo, ExpectedVersion: *input.ExpectedVersion,
		SubjectKind: subjectKind, SubjectRef: subjectRef, TurnRef: turnRef,
		Spans: spans, IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		commitmentProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

type declineCommitmentInput struct {
	VersionNo *int64    `json:"version_no"`
	Outcome   *string   `json:"outcome"`
	TurnRef   *string   `json:"turn_ref"`
	Spans     *[]string `json:"spans"`
}

func (s *Server) declineCommitmentAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	commitmentID, err := id.Parse(r.PathValue("commitment_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input declineCommitmentInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	if input.VersionNo == nil || *input.VersionNo < 1 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле version_no обязательно (>= 1).", false, nil)
		return
	}
	outcome := stringOrEmpty(input.Outcome)
	if outcome == "" {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле outcome обязательно: declined или change_requested.", false, nil)
		return
	}
	turnRef, err := id.Parse(stringOrEmpty(input.TurnRef))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле turn_ref должно быть UUID принятого текстового события кейса.", false, nil)
		return
	}
	spans := []string{}
	if input.Spans != nil {
		spans = *input.Spans
	}
	correlation, _ := correlationFromRequest(r)
	outcomeCmd, err := s.cases.RecordCommitmentDecline(r.Context(), casesAPI.RecordCommitmentDeclineCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, VersionNo: *input.VersionNo, Outcome: outcome,
		TurnRef: turnRef, Spans: spans, IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		commitmentProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcomeCmd)
}

// ---------------------------------------------------------------------------
// The operator forms (PRG)
// ---------------------------------------------------------------------------

// commitmentSpanFields are the form span inputs (up to eight verbatim
// excerpts; the empty ones never travel).
var commitmentSpanFields = [8]string{"span_1", "span_2", "span_3", "span_4", "span_5", "span_6", "span_7", "span_8"}

// commitmentSpansOf collects the non-empty span inputs of the form.
func commitmentSpansOf(r *http.Request) []string {
	spans := []string{}
	for _, field := range commitmentSpanFields {
		if value := r.FormValue(field); value != "" {
			spans = append(spans, value)
		}
	}
	return spans
}

func (s *Server) createCommitmentForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	status := r.FormValue("status")
	scope := r.FormValue("scope")
	serviceItems := strings.Split(r.FormValue("service_items"), "\n")
	priceKind := r.FormValue("price_kind")
	priceAmount, amountErr := formInt64Value(r, "price_amount_cents")
	priceMin, minErr := formInt64Value(r, "price_min_cents")
	priceMax, maxErr := formInt64Value(r, "price_max_cents")
	if amountErr != nil || minErr != nil || maxErr != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Цена — целые копейки (сумма или границы диапазона)."})
		return
	}
	payee := r.FormValue("payee")
	warrantyRole := r.FormValue("warranty_role")
	nextStep := r.FormValue("next_step")
	consequences := r.FormValue("consequences")
	unknowns := []string{}
	for _, field := range casesDomain.CommitmentMandatoryTerms {
		if r.FormValue("unknown_"+field) == "on" {
			unknowns = append(unknowns, field)
		}
	}
	actions := []string{}
	for _, field := range casesDomain.CommitmentActions {
		if r.FormValue("action_"+field) == "on" {
			actions = append(actions, field)
		}
	}
	validUntil, err := parseFormTimeLoose(r.FormValue("valid_until"))
	if err != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Срок действия обязателен (дата и время; RFC 3339, будущее)."})
		return
	}
	limit, err := formInt64(r, "consumption_limit")
	if err != nil || limit < 1 || limit > 10 {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Лимит потребления — целое 1..10."})
		return
	}
	commitmentID := id.UUID("")
	if value := r.FormValue("commitment_id"); value != "" {
		if commitmentID, err = id.Parse(value); err != nil {
			commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Идентификатор обязательства — UUID (новая версия = тот же идентификатор)."})
			return
		}
	}
	terms := casesDomain.MaterialTerms{
		Scope: scope, ServiceItems: serviceItems,
		Price: casesDomain.CommitmentPrice{Kind: priceKind, AmountCents: priceAmount, MinCents: priceMin, MaxCents: priceMax},
		Payee: payee, WarrantyRole: warrantyRole, NextStep: nextStep, Consequences: consequences,
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.cases.CreateCommitment(r.Context(), casesAPI.CreateCommitmentCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, Status: status, Terms: terms, ActionSet: actions,
		ValidUntil: validUntil, ConsumptionLimit: int(limit), MaterialUnknowns: unknowns,
		IdempotencyKey: formCommandKey("create-commitment", actor.ID.String(), caseID.String(), workItemID.String(), commitmentID.String(), status, scope, priceKind, strconv.FormatInt(priceAmount, 10), strconv.FormatInt(priceMin, 10), strconv.FormatInt(priceMax, 10), r.FormValue("valid_until"), strconv.FormatInt(limit, 10)),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, err)
		return
	}
	http.Redirect(w, r, commitmentReviewTarget(tenant, caseID, workItemID)+"?commitment_created=1", http.StatusSeeOther)
}

func (s *Server) confirmCommitmentForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	commitmentID, err := id.Parse(r.PathValue("commitment_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	if !formParse(w, r) {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	versionNo, err := formInt64(r, "version_no")
	if err != nil || versionNo < 1 {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Точная показанная версия обязательства обязательна (>= 1)."})
		return
	}
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Ожидаемая версия элемента работы (CAS) обязательна."})
		return
	}
	subjectRef := r.FormValue("subject_ref")
	if subjectRef == "" {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Субъект подтверждения (ссылка клиента) обязателен."})
		return
	}
	turnRef, err := formUUID(r, "turn_ref")
	if err != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите событие-источник согласия (принятый текстовый turn кейса)."})
		return
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.cases.ConfirmCommitment(r.Context(), casesAPI.ConfirmCommitmentCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, VersionNo: versionNo, ExpectedVersion: expected,
		SubjectKind: "client", SubjectRef: subjectRef, TurnRef: turnRef,
		Spans:          commitmentSpansOf(r),
		IdempotencyKey: formCommandKey("confirm-commitment", actor.ID.String(), caseID.String(), workItemID.String(), commitmentID.String(), strconv.FormatInt(versionNo, 10), strconv.FormatInt(expected, 10), subjectRef, turnRef.String()),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, err)
		return
	}
	http.Redirect(w, r, commitmentReviewTarget(tenant, caseID, workItemID)+"?commitment_confirmed=1", http.StatusSeeOther)
}

func (s *Server) declineCommitmentForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	commitmentID, err := id.Parse(r.PathValue("commitment_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	if !formParse(w, r) {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	versionNo, err := formInt64(r, "version_no")
	if err != nil || versionNo < 1 {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Точная показанная версия обязательства обязательна (>= 1)."})
		return
	}
	outcome := r.FormValue("outcome")
	turnRef, err := formUUID(r, "turn_ref")
	if err != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите событие-источник отказа (принятый текстовый turn кейса)."})
		return
	}
	correlation, _ := correlationFromRequest(r)
	_, err = s.cases.RecordCommitmentDecline(r.Context(), casesAPI.RecordCommitmentDeclineCommand{
		Actor: actor, CorrelationID: correlation, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, VersionNo: versionNo, Outcome: outcome,
		TurnRef: turnRef, Spans: commitmentSpansOf(r),
		IdempotencyKey: formCommandKey("decline-commitment", actor.ID.String(), caseID.String(), workItemID.String(), commitmentID.String(), strconv.FormatInt(versionNo, 10), outcome, turnRef.String()),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		commitmentFormError(w, r, tenant, caseID, workItemID, err)
		return
	}
	http.Redirect(w, r, commitmentReviewTarget(tenant, caseID, workItemID)+"?commitment_declined=1", http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// The Commitment Review page (the operator surface of one Work Item)
// ---------------------------------------------------------------------------

// commitmentVersionRow is one version card of the review page: the status
// word, the material conditions as text (the review scope — the case
// content), the action set, the validity window, the honest unknown holes,
// the content binding presence, the presentation link, the readiness
// derivation and the current flag (a superseded version is stated in TEXT —
// never by the color alone).
type commitmentVersionRow struct {
	CommitmentID  string
	No            string
	StatusText    string
	ReadinessText string
	CurrentText   string
	Scope         string
	ServiceItems  string
	PriceText     string
	Payee         string
	WarrantyRole  string
	NextStep      string
	Consequences  string
	ValidUntil    string
	ActionsText   string
	UnknownsText  string
	TokenText     string
	PresentedText string
	CreatedAt     string
}

// commitmentConfirmationRow is one confirmation card: the subject, the
// method, the expiry, the evidence turn and the read-time validity word.
type commitmentConfirmationRow struct {
	CommitmentID   string
	VersionNo      string
	ConfirmationID string
	Subject        string
	Method         string
	Channel        string
	ExpiresAt      string
	ValidityText   string
	TurnRef        string
	ConfirmedAt    string
}

// commitmentDeclineRow is one decline card: the outcome word, the evidence
// turn and the time (the decline mutates nothing — the honest history).
type commitmentDeclineRow struct {
	CommitmentID string
	VersionNo    string
	OutcomeText  string
	TurnRef      string
	CreatedAt    string
}

// commitmentReviewPageView is the page model of the Commitment Review.
type commitmentReviewPageView struct {
	CaseID         string
	CaseNo         string
	WorkItemID     string
	StateText      string
	Version        string
	Terminal       bool
	CanManageWork  bool
	PanelText      string
	Flash          string
	ChangedText    string
	CreatedFlag    bool
	ConfirmedFlag  bool
	DeclinedFlag   bool
	Versions       []commitmentVersionRow
	Confirmations  []commitmentConfirmationRow
	Declines       []commitmentDeclineRow
	StatusOptions  []workOption
	ActionOptions  []workOption
	DeclineOptions []workOption
	EventOptions   []workOption
	SubjectHint    string
	BackURL        string
}

// commitmentReviewPage renders the Commitment Review page of one Work Item:
// the version history with the readiness derivations, the confirmations
// with the validity derivations, the declines and the three PRG forms. The
// version changes are announced with aria-live text and a focus transfer to
// the description (AC-12); the confirm button is never focused nor
// preselected; the layout survives 320 px without a horizontal scroll.
func (s *Server) commitmentReviewPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	workItemID, err := id.Parse(r.PathValue("work_item_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return
	}
	view, err := s.cases.CaseView(r.Context(), actor, tenant, caseID)
	if err != nil {
		s.casesPageProblem(w, r, err)
		return
	}
	flash := flashError(r, tenant)
	if flash != "" {
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	page := commitmentReviewPageView{
		CaseID: caseID.String(), CaseNo: strconv.FormatInt(view.Case.CaseNo, 10),
		WorkItemID: workItemID.String(), CanManageWork: view.CanManageWork,
		PanelText: view.Commitments.PanelText, Flash: flash,
		StatusOptions:  dictionaryOptions(view.Commitments.StatusWords, commitmentStatusText),
		ActionOptions:  dictionaryOptions(view.Commitments.ActionWords, commitmentActionText),
		DeclineOptions: dictionaryOptions(view.Commitments.DeclineWords, commitmentDeclineText),
		SubjectHint:    "Субъект подтверждения — ссылка клиента (совпадает с проверенной личностью шага 2-3)",
		BackURL:        workFormTarget(tenant, caseID),
	}
	page.CreatedFlag = r.URL.Query().Get("commitment_created") == "1"
	page.ConfirmedFlag = r.URL.Query().Get("commitment_confirmed") == "1"
	page.DeclinedFlag = r.URL.Query().Get("commitment_declined") == "1"
	if page.CreatedFlag {
		page.ChangedText = "Версия обязательства создана: показана актуальная история версий."
	}
	if page.ConfirmedFlag {
		page.ChangedText = "Подтверждение записано: элемент работы переведён в состояние confirmed."
	}
	if page.DeclinedFlag {
		page.ChangedText = "Отказ записан: материальные условия не изменились."
	}
	var section casesAPI.CommitmentWorkItemView
	for _, item := range view.Commitments.Items {
		if item.WorkItemID == workItemID {
			section = item
			break
		}
	}
	for _, item := range view.WorkItems {
		if item.WorkItemID == workItemID {
			page.StateText = workItemStateText(item.State)
			page.Version = strconv.FormatInt(item.Version, 10)
			page.Terminal = item.Terminal
			break
		}
	}
	for _, card := range section.Versions {
		var terms casesDomain.MaterialTerms
		_ = json.Unmarshal(card.Version.Conditions, &terms)
		page.Versions = append(page.Versions, commitmentVersionRow{
			CommitmentID:  card.Version.CommitmentID.String(),
			No:            strconv.FormatInt(card.Version.VersionNo, 10),
			StatusText:    commitmentStatusText(card.Version.Status),
			ReadinessText: commitmentReadinessText(card.Readiness),
			CurrentText:   map[bool]string{true: "текущая", false: "заменена новой версией"}[card.Current],
			Scope:         terms.Scope, ServiceItems: strings.Join(terms.ServiceItems, "; "),
			PriceText: commitmentPriceText(terms.Price),
			Payee:     terms.Payee, WarrantyRole: terms.WarrantyRole,
			NextStep: terms.NextStep, Consequences: terms.Consequences,
			ValidUntil:    formatTime(card.Version.ValidUntil),
			ActionsText:   strings.Join(card.Version.ActionSet, ", "),
			UnknownsText:  unknownsText(card.Version.MaterialUnknowns),
			TokenText:     map[bool]string{true: "привязка контента записана (HMAC)", false: "нет токена"}[card.Version.ContentToken != ""],
			PresentedText: map[bool]string{true: "предъявлена клиенту", false: "не предъявлена"}[card.Presented],
			CreatedAt:     formatTime(card.Version.CreatedAt),
		})
	}
	for _, confirmation := range section.Confirmations {
		page.Confirmations = append(page.Confirmations, commitmentConfirmationRow{
			CommitmentID:   confirmation.Record.CommitmentID.String(),
			VersionNo:      strconv.FormatInt(confirmation.Record.VersionNo, 10),
			ConfirmationID: confirmation.Record.ConfirmationID.String(),
			Subject:        confirmation.Record.SubjectKind + "/" + confirmation.Record.SubjectRef,
			Method:         confirmation.Record.Method, Channel: confirmation.Record.Channel,
			ExpiresAt:    formatTime(confirmation.Record.ExpiresAt),
			ValidityText: confirmationValidityText(confirmation.Validity),
			TurnRef:      confirmation.Record.TurnRef.String(),
			ConfirmedAt:  formatTime(confirmation.Record.ConfirmedAt),
		})
	}
	for _, decline := range section.Declines {
		page.Declines = append(page.Declines, commitmentDeclineRow{
			CommitmentID: decline.CommitmentID.String(),
			VersionNo:    strconv.FormatInt(decline.VersionNo, 10),
			OutcomeText:  commitmentDeclineText(decline.Outcome),
			TurnRef:      decline.TurnRef.String(),
			CreatedAt:    formatTime(decline.CreatedAt),
		})
	}
	// The turn select options of the evidence forms: the accepted text
	// events of the case (the same honest list as the objection form).
	page.EventOptions = eventOptionsOf(view)
	s.render(w, "commitment", map[string]any{"TenantID": tenant, "View": page})
}

// dictionaryOptions renders the fixed dictionary select options.
func dictionaryOptions(words []string, text func(string) string) []workOption {
	out := make([]workOption, 0, len(words))
	for _, word := range words {
		out = append(out, workOption{Value: word, Text: text(word)})
	}
	return out
}

// commitmentStatusText renders the honest Russian status word.
func commitmentStatusText(status string) string {
	switch status {
	case casesDomain.CommitmentStatusDraft:
		return "черновик (не подтверждаем)"
	case casesDomain.CommitmentStatusOffered:
		return "предложена (offered)"
	}
	return status
}

// commitmentReadinessText renders the honest readiness derivation word.
func commitmentReadinessText(readiness string) string {
	switch readiness {
	case casesDomain.CommitmentReadinessDraft:
		return "черновик — не подтверждаем"
	case casesDomain.CommitmentReadinessUnknownBlocked:
		return "неизвестный обязательный термин блокирует готовность"
	case casesDomain.CommitmentReadinessConfirmable:
		return "подтверждаема"
	case casesDomain.CommitmentReadinessSuperseded:
		return "заменена новой версией"
	}
	return readiness
}

// confirmationValidityText renders the honest validity word (the reserved
// words are never produced — the derivation is read-time).
func confirmationValidityText(validity string) string {
	switch validity {
	case casesDomain.ConfirmationValidityValid:
		return "действует"
	case casesDomain.ConfirmationValidityExpired:
		return "истекла (срок версии прошёл)"
	case casesDomain.ConfirmationValiditySuperseded:
		return "заменена (есть версия новее)"
	case casesDomain.ConfirmationValidityOwnerStale:
		return "владение изменилось"
	case casesDomain.ConfirmationValidityReleaseStale:
		return "релиз изменился"
	}
	return validity
}

// commitmentDeclineText renders the honest outcome word.
func commitmentDeclineText(outcome string) string {
	switch outcome {
	case casesDomain.CommitmentDeclineOutcomeDeclined:
		return "отказ"
	case casesDomain.CommitmentDeclineOutcomeChangeRequested:
		return "запрос изменения"
	}
	return outcome
}

// commitmentActionText renders the honest action word (the execution does
// not exist as a class — Epic 3).
func commitmentActionText(action string) string {
	switch action {
	case casesDomain.CommitmentActionPrepareOrder:
		return "подготовить заказ (prepare_order)"
	case casesDomain.CommitmentActionScheduleAppointment:
		return "записать на приём (schedule_appointment)"
	case casesDomain.CommitmentActionIssueDocument:
		return "выпустить документ (issue_document)"
	}
	return action
}

// commitmentPriceText renders the price term (the amount or the honest
// range, in kopecks).
func commitmentPriceText(price casesDomain.CommitmentPrice) string {
	switch price.Kind {
	case casesDomain.CommitmentPriceKindAmount:
		return "сумма " + strconv.FormatInt(price.AmountCents, 10) + " коп."
	case casesDomain.CommitmentPriceKindRange:
		return "диапазон " + strconv.FormatInt(price.MinCents, 10) + "–" + strconv.FormatInt(price.MaxCents, 10) + " коп."
	}
	if price.Kind == "" {
		return "цена неизвестна"
	}
	return price.Kind
}

// unknownsText renders the honest unknown holes list.
func unknownsText(unknowns []string) string {
	if len(unknowns) == 0 {
		return "нет (все обязательные термины известны)"
	}
	return "неизвестны: " + strings.Join(unknowns, ", ")
}

// eventOptionsOf renders the accepted event options of the case (the
// evidence turn selects; the same honest list as the objection form — the
// modality boundary is the command's own honest refusal).
func eventOptionsOf(view casesAPI.CaseView) []workOption {
	out := []workOption{}
	for _, event := range view.Timeline {
		if event.State == "accepted" {
			out = append(out, workOption{
				Value: event.EventID.String(),
				Text:  event.Channel + " · " + formatTime(event.ReceivedAt) + " · " + event.EventID.String(),
			})
		}
	}
	return out
}
