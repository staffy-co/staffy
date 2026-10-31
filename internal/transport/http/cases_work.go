package httptransport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/httpx"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The work command surface of Story 2.2: the machine JSON contract (the
// strict per-command DTOs with the Idempotency-Key header) and the operator
// forms of the Work Control Card (the PRG pattern — every form returns to
// the case page with the honest flash).

// workBodyLimit bounds the command bodies: the fact values are dictionary
// bounded (2 KiB) and no command carries a payload; 64 KiB is a generous
// transport cap.
const workBodyLimit = 64 * 1024

// formCommandKey derives the deterministic idempotency key of one form
// submission: the same fields replay the same command; the material
// guards (the version expectations, the fact material tuple) stay the
// concurrency truth.
func formCommandKey(kind string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(append([]string{kind}, parts...), "\x1f")))
	return "form-" + hex.EncodeToString(sum[:16])
}

func (s *Server) workCommander(w http.ResponseWriter, r *http.Request) bool {
	if s.cases == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// The machine JSON contract
// ---------------------------------------------------------------------------

type createWorkItemInput struct {
	ConversationID *string `json:"conversation_id"`
	Type           *string `json:"type"`
	Title          *string `json:"title"`
	Reason         *string `json:"reason"`
}

type classifyWorkItemInput struct {
	ExpectedVersion *int64  `json:"expected_version"`
	Type            *string `json:"type"`
	Title           *string `json:"title"`
	Reason          *string `json:"reason"`
}

type transitionWorkItemInput struct {
	ExpectedVersion *int64  `json:"expected_version"`
	ToState         *string `json:"to_state"`
	Reason          *string `json:"reason"`
}

type splitTargetInput struct {
	Type  *string `json:"type"`
	Title *string `json:"title"`
}

type splitWorkItemInput struct {
	ExpectedVersion *int64              `json:"expected_version"`
	Targets         *[]splitTargetInput `json:"targets"`
	Reason          *string             `json:"reason"`
}

type mergeSourceInput struct {
	WorkItemID      *string `json:"work_item_id"`
	ExpectedVersion *int64  `json:"expected_version"`
}

type mergeWorkItemInput struct {
	ExpectedVersion *int64              `json:"expected_version"`
	Sources         *[]mergeSourceInput `json:"sources"`
	Reason          *string             `json:"reason"`
}

type linkEventInput struct {
	ExpectedVersion *int64  `json:"expected_version"`
	EventID         *string `json:"event_id"`
}

type disputeWorkItemInput struct {
	ExpectedVersion *int64  `json:"expected_version"`
	Note            *string `json:"note"`
}

type recordFactInput struct {
	WorkItemID  *string          `json:"work_item_id"`
	MaterialKey *string          `json:"material_key"`
	Value       *json.RawMessage `json:"value"`
	SourceType  *string          `json:"source_type"`
	SourceID    *string          `json:"source_id"`
	Reason      *string          `json:"reason"`
}

type confirmFactInput struct {
	VersionNo       *int64  `json:"version_no"`
	ConfirmerType   *string `json:"confirmer_type"`
	EvidenceEventID *string `json:"evidence_event_id"`
	Note            *string `json:"note"`
}

type rescopeFactInput struct {
	TargetWorkItem  *string `json:"target_work_item_id"`
	ExpectedVersion *int64  `json:"expected_version"`
	Reason          *string `json:"reason"`
}

// decodeWorkBody strictly decodes one JSON command body.
func decodeWorkBody(w http.ResponseWriter, r *http.Request, target any) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, workBodyLimit))
	if err != nil {
		writeWorkProblem(w, r, "invalid_request", "Тело команды не читается.")
		return false
	}
	decoder := json.NewDecoder(bytesReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeWorkProblem(w, r, "invalid_request", "Проверьте поля команды: строгий разбор (неизвестные поля запрещены).")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeWorkProblem(w, r, "invalid_request", "Тело команды должно содержать один JSON-объект.")
		return false
	}
	return true
}

// workKey reads and validates the Idempotency-Key header of one command.
func workKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) < 8 || len(key) > 200 {
		writeWorkProblem(w, r, "invalid_request", "Заголовок Idempotency-Key обязателен (8–200 символов).")
		return "", false
	}
	return key, true
}

// writeWorkOutcome answers one accepted (or replayed) command.
func writeWorkOutcome(w http.ResponseWriter, r *http.Request, outcome casesAPI.WorkOutcome) {
	status := http.StatusCreated
	if outcome.Replay {
		status = http.StatusOK
		w.Header().Set("Idempotency-Replayed", "true")
	}
	writeJSON(w, status, outcome)
}

// workProblem maps the command errors onto the problem contract: the rule
// refusals are 400, the optimistic and key conflicts are 409, a foreign
// tenant is a plain 404.
func workProblem(w http.ResponseWriter, r *http.Request, server *Server, err error) {
	var rule casesDomain.WorkRuleError
	switch {
	case errors.As(err, &rule):
		server.problem(w, r, http.StatusBadRequest, rule.Rule, rule.Reason, false, nil)
	case errors.Is(err, casesDomain.ErrStaleVersion):
		server.problem(w, r, http.StatusConflict, "stale_version", "Материал изменился: сверьте текущую версию и повторите команду.", false, nil)
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		server.problem(w, r, http.StatusConflict, "command_key_conflict", "Идемпотентный ключ уже использован другой командой: исход не воспроизводится для другого материала.", false, nil)
	case errors.Is(err, casesDomain.ErrActiveBinding):
		server.problem(w, r, http.StatusConflict, "active_binding", "У разговора уже есть активная связь: отмените её прежде чем связывать снова.", false, nil)
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		server.problem(w, r, http.StatusNotFound, "not_found", "Обращение, элемент или факт не найдены.", false, nil)
	case errors.Is(err, domain.ErrForbidden):
		server.problem(w, r, http.StatusForbidden, "cases_forbidden", "Управление рабочими элементами требует полномочия review.", false, nil)
	default:
		correlationID, _ := correlationFromRequest(r)
		server.logger.ErrorContext(r.Context(), "work command failed", "code", "work_command_failed", "correlation_id", correlationID.String(), "error", err.Error())
		server.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось выполнить команду. Повторите позднее.", true, nil)
	}
}

// writeWorkProblem writes one transport-level work problem without the
// server reference (the plain field refusals of the strict decode).
func writeWorkProblem(w http.ResponseWriter, r *http.Request, code, message string) {
	correlationID, _ := correlationFromRequest(r)
	httpx.WriteProblem(w, http.StatusBadRequest, httpx.Problem{
		Code: code, Message: message, CorrelationID: correlationID.String(),
	})
}

func (s *Server) createWorkItemAPI(w http.ResponseWriter, r *http.Request) {
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
	var input createWorkItemInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	conversationID, err := id.Parse(stringOrEmpty(input.ConversationID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле conversation_id должно быть UUID.", false, nil)
		return
	}
	outcome, err := s.cases.CreateWorkItem(r.Context(), casesAPI.CreateWorkItemCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Type: stringOrEmpty(input.Type), Title: stringOrEmpty(input.Title), Reason: stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) classifyWorkItemAPI(w http.ResponseWriter, r *http.Request) {
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
	var input classifyWorkItemInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	outcome, err := s.cases.ClassifyWorkItem(r.Context(), casesAPI.ClassifyCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: int64OrZero(input.ExpectedVersion), Reason: stringOrEmpty(input.Reason),
			IdempotencyKey: key, Now: s.clock.Now().UTC(),
		},
		Type: stringOrEmpty(input.Type), Title: stringOrEmpty(input.Title),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) transitionWorkItemAPI(w http.ResponseWriter, r *http.Request) {
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
	var input transitionWorkItemInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	outcome, err := s.cases.TransitionWorkItem(r.Context(), casesAPI.TransitionCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: int64OrZero(input.ExpectedVersion), Reason: stringOrEmpty(input.Reason),
			IdempotencyKey: key, Now: s.clock.Now().UTC(),
		},
		ToState: stringOrEmpty(input.ToState),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) splitWorkItemAPI(w http.ResponseWriter, r *http.Request) {
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
	var input splitWorkItemInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	targets := []casesDomain.SplitTarget{}
	if input.Targets != nil {
		for _, target := range *input.Targets {
			targets = append(targets, casesDomain.SplitTarget{Type: stringOrEmpty(target.Type), Title: stringOrEmpty(target.Title)})
		}
	}
	outcome, err := s.cases.SplitWorkItem(r.Context(), casesAPI.SplitCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: int64OrZero(input.ExpectedVersion), Reason: stringOrEmpty(input.Reason),
			IdempotencyKey: key, Now: s.clock.Now().UTC(),
		},
		Targets: targets,
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) mergeWorkItemAPI(w http.ResponseWriter, r *http.Request) {
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
	var input mergeWorkItemInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	sources := []casesAPI.MergeSourceInput{}
	if input.Sources != nil {
		for _, source := range *input.Sources {
			sourceID, err := id.Parse(stringOrEmpty(source.WorkItemID))
			if err != nil {
				s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле sources[].work_item_id должно быть UUID.", false, nil)
				return
			}
			sources = append(sources, casesAPI.MergeSourceInput{WorkItemID: sourceID, ExpectedVersion: int64OrZero(source.ExpectedVersion)})
		}
	}
	outcome, err := s.cases.MergeWorkItems(r.Context(), casesAPI.MergeCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: int64OrZero(input.ExpectedVersion), Reason: stringOrEmpty(input.Reason),
			IdempotencyKey: key, Now: s.clock.Now().UTC(),
		},
		Sources: sources,
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) linkEventAPI(w http.ResponseWriter, r *http.Request) {
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
	var input linkEventInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	eventID, err := id.Parse(stringOrEmpty(input.EventID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле event_id должно быть UUID.", false, nil)
		return
	}
	outcome, err := s.cases.LinkEvent(r.Context(), casesAPI.LinkEventCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: int64OrZero(input.ExpectedVersion),
			IdempotencyKey:  key, Now: s.clock.Now().UTC(),
		},
		EventID: eventID,
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) disputeWorkItemAPI(w http.ResponseWriter, r *http.Request) {
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
	var input disputeWorkItemInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	outcome, err := s.cases.DisputeWorkItem(r.Context(), casesAPI.DisputeCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: int64OrZero(input.ExpectedVersion),
			IdempotencyKey:  key, Now: s.clock.Now().UTC(),
		},
		Note: stringOrEmpty(input.Note),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) recordFactAPI(w http.ResponseWriter, r *http.Request) {
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
	var input recordFactInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	workItemID, err := id.Parse(stringOrEmpty(input.WorkItemID))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле work_item_id должно быть UUID.", false, nil)
		return
	}
	if input.Value == nil || len(*input.Value) == 0 {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле value обязательно (одно JSON-значение).", false, nil)
		return
	}
	sourceType := stringOrEmpty(input.SourceType)
	sourceID := stringOrEmpty(input.SourceID)
	if sourceType == casesDomain.FactSourceOperator {
		// The operator source id IS the command key (the 2-2 provenance
		// contract: the operator may fix values — a new key is a new
		// material source).
		sourceID = key
	}
	outcome, err := s.cases.RecordFact(r.Context(), casesAPI.RecordFactCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: stringOrEmpty(input.MaterialKey), Value: *input.Value,
		SourceType: sourceType, SourceID: sourceID, Reason: stringOrEmpty(input.Reason),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) confirmFactAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, factID, ok := s.casesFactActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input confirmFactInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	evidenceEventID := id.UUID("")
	if evidence := stringOrEmpty(input.EvidenceEventID); evidence != "" {
		parsed, err := id.Parse(evidence)
		if err != nil {
			s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле evidence_event_id должно быть UUID.", false, nil)
			return
		}
		evidenceEventID = parsed
	}
	confirmer := stringOrEmpty(input.ConfirmerType)
	if confirmer == "" {
		confirmer = casesDomain.ConfirmerOperatorRelay
	}
	outcome, err := s.cases.ConfirmFactVersion(r.Context(), casesAPI.ConfirmFactVersionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, FactID: factID,
		VersionNo: int64OrZero(input.VersionNo), ConfirmerType: confirmer,
		EvidenceEventID: evidenceEventID, Note: stringOrEmpty(input.Note),
		IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) rescopeFactAPI(w http.ResponseWriter, r *http.Request) {
	if !s.workCommander(w, r) {
		return
	}
	actor, tenant, caseID, factID, ok := s.casesFactActor(w, r)
	if !ok {
		return
	}
	key, ok := workKey(w, r)
	if !ok {
		return
	}
	var input rescopeFactInput
	if !decodeWorkBody(w, r, &input) {
		return
	}
	target, err := id.Parse(stringOrEmpty(input.TargetWorkItem))
	if err != nil {
		s.problem(w, r, http.StatusBadRequest, "invalid_request", "Поле target_work_item_id должно быть UUID.", false, nil)
		return
	}
	outcome, err := s.cases.RescopeFact(r.Context(), casesAPI.RescopeFactCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, FactID: factID,
		TargetWorkItem: target, ExpectedVersion: int64OrZero(input.ExpectedVersion),
		Reason: stringOrEmpty(input.Reason), IdempotencyKey: key, Now: s.clock.Now().UTC(),
	})
	if err != nil {
		workProblem(w, r, s, err)
		return
	}
	writeWorkOutcome(w, r, outcome)
}

func (s *Server) factsViewAPI(w http.ResponseWriter, r *http.Request) {
	if s.cases == nil {
		s.problem(w, r, http.StatusServiceUnavailable, "cases_unavailable", "Контур обращений не сконфигурирован.", false, nil)
		return
	}
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	view, err := s.cases.FactsView(r.Context(), actor, tenant, caseID)
	if err != nil {
		casesProblem(w, r, s, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// ---------------------------------------------------------------------------
// The operator forms (the PRG pattern of the release surface)
// ---------------------------------------------------------------------------

// casesWorkActor resolves the actor, the tenant, the case and the work item
// of one work-item-scoped request.
func (s *Server) casesWorkActor(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, id.UUID, bool) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return domain.Principal{}, id.UUID(""), id.UUID(""), id.UUID(""), false
	}
	workItemID, err := id.Parse(r.PathValue("work_item_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return domain.Principal{}, id.UUID(""), id.UUID(""), id.UUID(""), false
	}
	return actor, tenant, caseID, workItemID, true
}

// casesFactActor resolves the actor, the tenant, the case and the fact of
// one fact-scoped request.
func (s *Server) casesFactActor(w http.ResponseWriter, r *http.Request) (domain.Principal, id.UUID, id.UUID, id.UUID, bool) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return domain.Principal{}, id.UUID(""), id.UUID(""), id.UUID(""), false
	}
	factID, err := id.Parse(r.PathValue("fact_id"))
	if err != nil {
		s.notFound(w, r, actor)
		return domain.Principal{}, id.UUID(""), id.UUID(""), id.UUID(""), false
	}
	return actor, tenant, caseID, factID, true
}

// workFormTarget is the PRG target of every work form: the case page.
func workFormTarget(tenant, caseID id.UUID) string {
	return "/app/tenants/" + tenant.String() + "/cases/" + caseID.String()
}

// workFormError flashes the honest message and returns to the case page.
func workFormError(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, err error) {
	var rule casesDomain.WorkRuleError
	message := "Действие не выполнено. Повторите позднее."
	switch {
	case errors.As(err, &rule):
		message = rule.Reason
	case errors.Is(err, casesDomain.ErrStaleVersion):
		message = "Материал изменился: страница показывает актуальное состояние — сверьте версию и повторите."
	case errors.Is(err, casesDomain.ErrCommandKeyConflict):
		message = "Ключ команды уже использован другим материалом: команда не выполнена."
	case errors.Is(err, casesDomain.ErrActiveBinding):
		message = "У разговора уже есть активная связь: отмените её прежде чем связывать снова."
	case errors.Is(err, casesDomain.ErrNotFound), errors.Is(err, domain.ErrNotFound):
		message = "Обращение, элемент или факт не найдены."
	case errors.Is(err, domain.ErrForbidden):
		message = "Управление рабочими элементами требует полномочия review."
	}
	setFlashError(w, tenant, message)
	http.Redirect(w, r, workFormTarget(tenant, caseID), http.StatusSeeOther)
}

// workFormDone returns to the case page with the success flag.
func workFormDone(w http.ResponseWriter, r *http.Request, tenant, caseID id.UUID, flag string) {
	http.Redirect(w, r, workFormTarget(tenant, caseID)+"?"+flag+"=1", http.StatusSeeOther)
}

// formParse parses the bounded form body.
func formParse(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	return r.ParseForm() == nil
}

func formInt64(r *http.Request, field string) (int64, error) {
	value := strings.TrimSpace(r.FormValue(field))
	if value == "" {
		return 0, errors.New("версия элемента обязательна")
	}
	return strconv.ParseInt(value, 10, 64)
}

func formUUID(r *http.Request, field string) (id.UUID, error) {
	return id.Parse(strings.TrimSpace(r.FormValue(field)))
}

// formOption splits the "<uuid>:<version>" option value of the merge form.
func formOption(value string) (id.UUID, int64, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 {
		return id.UUID(""), 0, errors.New("выберите источник объединения")
	}
	parsed, err := id.Parse(parts[0])
	if err != nil {
		return id.UUID(""), 0, err
	}
	version, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return id.UUID(""), 0, err
	}
	return parsed, version, nil
}

func (s *Server) createWorkItemForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	conversationID, err := formUUID(r, "conversation_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите диалог нового намерения."})
		return
	}
	workType, title, reason := r.FormValue("type"), r.FormValue("title"), r.FormValue("reason")
	_, err = s.cases.CreateWorkItem(r.Context(), casesAPI.CreateWorkItemCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, ConversationID: conversationID,
		Type: workType, Title: title, Reason: reason,
		IdempotencyKey: formCommandKey("create", actor.ID.String(), caseID.String(), conversationID.String(), workType, title, reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "work_item_created")
}

func (s *Server) classifyWorkItemForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия элемента обязательна."})
		return
	}
	workType, title, reason := r.FormValue("type"), r.FormValue("title"), r.FormValue("reason")
	_, err = s.cases.ClassifyWorkItem(r.Context(), casesAPI.ClassifyCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: expected, Reason: reason,
			IdempotencyKey: formCommandKey("classify", actor.ID.String(), caseID.String(), workItemID.String(), strconv.FormatInt(expected, 10), workType, title, reason),
		},
		Type: workType, Title: title,
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "classified")
}

func (s *Server) transitionWorkItemForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия элемента обязательна."})
		return
	}
	toState, reason := r.FormValue("to_state"), r.FormValue("reason")
	_, err = s.cases.TransitionWorkItem(r.Context(), casesAPI.TransitionCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: expected, Reason: reason,
			IdempotencyKey: formCommandKey("transition", actor.ID.String(), caseID.String(), workItemID.String(), strconv.FormatInt(expected, 10), toState, reason),
		},
		ToState: toState,
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "transitioned")
}

func (s *Server) splitWorkItemForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия элемента обязательна."})
		return
	}
	targets := []casesDomain.SplitTarget{
		{Type: r.FormValue("target_1_type"), Title: r.FormValue("target_1_title")},
		{Type: r.FormValue("target_2_type"), Title: r.FormValue("target_2_title")},
	}
	reason := r.FormValue("reason")
	_, err = s.cases.SplitWorkItem(r.Context(), casesAPI.SplitCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: expected, Reason: reason,
			IdempotencyKey: formCommandKey("split", actor.ID.String(), caseID.String(), workItemID.String(), strconv.FormatInt(expected, 10), r.FormValue("target_1_type"), r.FormValue("target_1_title"), r.FormValue("target_2_type"), r.FormValue("target_2_title"), reason),
		},
		Targets: targets,
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "split")
}

func (s *Server) mergeWorkItemForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия элемента обязательна."})
		return
	}
	sourceID, sourceVersion, err := formOption(r.FormValue("source"))
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: err.Error()})
		return
	}
	reason := r.FormValue("reason")
	_, err = s.cases.MergeWorkItems(r.Context(), casesAPI.MergeCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: expected, Reason: reason,
			IdempotencyKey: formCommandKey("merge", actor.ID.String(), caseID.String(), workItemID.String(), strconv.FormatInt(expected, 10), sourceID.String(), strconv.FormatInt(sourceVersion, 10), reason),
		},
		Sources: []casesAPI.MergeSourceInput{{WorkItemID: sourceID, ExpectedVersion: sourceVersion}},
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "merged")
}

func (s *Server) linkEventForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия элемента обязательна."})
		return
	}
	eventID, err := formUUID(r, "event_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите событие."})
		return
	}
	_, err = s.cases.LinkEvent(r.Context(), casesAPI.LinkEventCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: expected,
			IdempotencyKey:  formCommandKey("link", actor.ID.String(), caseID.String(), workItemID.String(), strconv.FormatInt(expected, 10), eventID.String()),
		},
		EventID: eventID,
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "linked")
}

func (s *Server) disputeWorkItemForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, workItemID, ok := s.casesWorkActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия элемента обязательна."})
		return
	}
	note := r.FormValue("note")
	_, err = s.cases.DisputeWorkItem(r.Context(), casesAPI.DisputeCommand{
		WorkCommandInput: casesAPI.WorkCommandInput{
			Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
			ExpectedVersion: expected,
			IdempotencyKey:  formCommandKey("dispute", actor.ID.String(), caseID.String(), workItemID.String(), strconv.FormatInt(expected, 10), note),
		},
		Note: note,
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "disputed")
}

func (s *Server) recordFactForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, ok := s.casesActorAndCase(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	workItemID, err := formUUID(r, "work_item_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите элемент-скоуп."})
		return
	}
	materialKey, value, reason := r.FormValue("material_key"), r.FormValue("value"), r.FormValue("reason")
	sourceType := r.FormValue("source_type")
	sourceID := r.FormValue("source_id")
	if sourceType == casesDomain.FactSourceOperator {
		sourceID = formCommandKey("fact", actor.ID.String(), caseID.String(), materialKey)
	}
	_, err = s.cases.RecordFact(r.Context(), casesAPI.RecordFactCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: materialKey, Value: []byte(value),
		SourceType: sourceType, SourceID: sourceID, Reason: reason,
		IdempotencyKey: formCommandKey("record-fact", actor.ID.String(), caseID.String(), materialKey, value, sourceType, sourceID),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "fact_recorded")
}

func (s *Server) confirmFactForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, factID, ok := s.casesFactActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	versionNo, err := formInt64(r, "version_no")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия факта обязательна."})
		return
	}
	evidenceEventID := id.UUID("")
	if evidence := strings.TrimSpace(r.FormValue("evidence_event_id")); evidence != "" {
		evidenceEventID, err = id.Parse(evidence)
		if err != nil {
			workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Evidence-событие должно быть UUID."})
			return
		}
	}
	_, err = s.cases.ConfirmFactVersion(r.Context(), casesAPI.ConfirmFactVersionCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, FactID: factID,
		VersionNo: versionNo, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		EvidenceEventID: evidenceEventID, Note: r.FormValue("note"),
		IdempotencyKey: formCommandKey("confirm", actor.ID.String(), caseID.String(), factID.String(), strconv.FormatInt(versionNo, 10), evidenceEventID.String(), r.FormValue("note")),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "confirmed")
}

func (s *Server) rescopeFactForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, caseID, factID, ok := s.casesFactActor(w, r)
	if !ok {
		return
	}
	if !formParse(w, r) {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	target, err := formUUID(r, "target_work_item_id")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Выберите целевой элемент."})
		return
	}
	reason := r.FormValue("reason")
	// The fact ladder height the page rendered: the honest expectation.
	expected, err := formInt64(r, "expected_version")
	if err != nil {
		workFormError(w, r, tenant, caseID, casesDomain.WorkRuleError{Rule: "form", Reason: "Версия факта обязательна."})
		return
	}
	_, err = s.cases.RescopeFact(r.Context(), casesAPI.RescopeFactCommand{
		Actor: actor, TenantID: tenant, CaseID: caseID, FactID: factID,
		TargetWorkItem: target, ExpectedVersion: expected, Reason: reason,
		IdempotencyKey: formCommandKey("rescope", actor.ID.String(), caseID.String(), factID.String(), target.String(), strconv.FormatInt(expected, 10), reason),
		Now:            s.clock.Now().UTC(),
	})
	if err != nil {
		workFormError(w, r, tenant, caseID, err)
		return
	}
	workFormDone(w, r, tenant, caseID, "rescoped")
}

func int64OrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

var _ = time.Now
