package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
)

func (s *Server) readinessService() employeeAPI.ReadinessService { return s.readiness }

// readinessScope resolves the principal plus the tenant and employee path
// values of the test & release surface.
func (s *Server) readinessScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return actor, "", "", false
	}
	employee, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return actor, "", "", false
	}
	return actor, tenant, employee, true
}

// readinessProblem maps the surface errors to honest problem responses:
// the gate race and the publication race carry the current state for an
// honest comparison; a rule rejection carries every concrete reason.
func (s *Server) readinessProblem(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
	fields := map[string]string{}
	var rule employeeDomain.ProfileRuleError
	var gateConflict employeeDomain.GateConflictError
	var admissionConflict employeeDomain.AdmissionConflictError
	switch {
	case errors.Is(err, employeeDomain.ErrNotFound):
		status, code, message = 404, "not_found", "Ресурс не найден."
	case errors.Is(err, employeeDomain.ErrReadinessForbidden):
		status, code, message = 403, "readiness_forbidden", "Test & Release доступен с полномочиями configuration, review или audit; команды — только configuration."
	case errors.Is(err, employeeDomain.ErrReadinessStepUpRequired):
		status, code, message = 403, "step_up_required", "Переходы gate-инстансов и публикация требуют step-up подтверждения: провайдер повторной аутентификации не подключён в этом срезе (честный отказ, не обход)."
	case errors.As(err, &gateConflict):
		status, code, message = 409, "gate_transition_conflict", "Переход инстанса уже применён другой командой: ровно один преемник записи. Перечитайте текущее состояние."
		fields["instance_id"] = gateConflict.InstanceID.String()
		fields["reason"] = gateConflict.Reason
	case errors.As(err, &admissionConflict):
		status, code, message = 409, "admission_conflict", "Решение этой стадии этого кандидата уже действует: ровно одна запись, повторная публикация не создаётся."
		fields["stage"] = admissionConflict.Stage
		if admissionConflict.Current != nil {
			fields["decision_no"] = strconv.FormatInt(admissionConflict.Current.DecisionNo, 10)
			fields["digest"] = admissionConflict.Current.Digest
			fields["admission_view"] = "GET /v1/tenants/{tenant_id}/employees/{employee_id}/test-release/admissions"
		}
	case errors.As(err, &rule):
		status, code, message = 400, "readiness_rule", rule.Reason
		fields["section"] = rule.Section
		fields["field"] = rule.Field
		if rule.Kind != "" {
			fields["kind"] = rule.Kind
		}
		for index, problem := range rule.Problems {
			fields["problem_"+strconv.Itoa(index)] = problem.Section + "." + problem.Field + " (" + problem.Kind + "): " + problem.Reason
		}
	case errors.Is(err, employeeDomain.ErrConflict):
		status, code, message = 409, "admission_conflict", "Конкурентная команда уже применилась: обновите состояние и повторите."
	case errors.Is(err, employeeDomain.ErrInvalid):
		status, code, message = 400, "invalid_readiness_command", "Проверьте параметры команды."
	}
	s.problem(w, r, status, code, message, status >= 500, fields)
}

// BindReadiness mounts the test & release surface: the JSON API and the
// accessible Test & Release Panel (Story 1-14).
func (s *Server) BindReadiness(service employeeAPI.ReadinessService) {
	s.readiness = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/test-release", s.readinessViewAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/test-release/gate-instances", s.gateInstancesViewAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/test-release/gate-instances", s.gateTransitionAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/test-release/admissions", s.admissionHistoryAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/test-release/admissions", s.publishAdmissionAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/employees/{employee_id}/test-release", s.testReleasePage)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/test-release/gate-instances", s.gateTransitionForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/test-release/admissions", s.publishAdmissionForm)
}

// decodeReadinessBody reads one strict JSON command body.
func decodeReadinessBody(w http.ResponseWriter, r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return employeeDomain.ProfileRuleError{Section: "request", Field: "content_type", Reason: "Ожидается application/json.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return employeeDomain.ProfileRuleError{Section: "request", Field: "body", Reason: "Тело команды не распознано: проверьте поля и типы.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	if err = d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return employeeDomain.ProfileRuleError{Section: "request", Field: "body", Reason: "Тело команды содержит лишние данные.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	return nil
}

// gateTransitionBody is the wire shape of the gate transition command.
type gateTransitionBody struct {
	DefinitionID string `json:"definition_id"`
	Route        *struct {
		Channel   string `json:"channel"`
		Direction string `json:"direction"`
	} `json:"route"`
	Modality   string `json:"modality"`
	State      string `json:"state"`
	Supersedes string `json:"supersedes_instance_id"`
	ApprovedBy []struct {
		ParticipantID string `json:"participant_id"`
		Role          string `json:"role"`
	} `json:"approved_by"`
	EvidenceRefs        []string `json:"evidence_refs"`
	EvidenceHash        string   `json:"evidence_hash"`
	NotApplicableReason string   `json:"not_applicable_reason"`
	ExpiryTrigger       string   `json:"expiry_trigger"`
}

// gateTransitionCommand validates the wire values and builds the command.
func gateTransitionCommand(s *Server, r *http.Request, actor identityDomain.Principal, tenant, employee id.UUID, body gateTransitionBody) (employeeAPI.GateTransitionCommand, error) {
	invalid := func(field, reason string) error {
		return employeeDomain.ProfileRuleError{Section: "gate_instance", Field: field, Reason: reason, Kind: employeeDomain.ProfileProblemInvalid}
	}
	command := employeeAPI.GateTransitionCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, DefinitionID: body.DefinitionID, Modality: body.Modality, TargetState: body.State, NotApplicableReason: body.NotApplicableReason, ExpiryTrigger: body.ExpiryTrigger}
	if body.Route != nil {
		command.Route = &employeeDomain.Route{Channel: body.Route.Channel, Direction: body.Route.Direction}
	}
	if body.Supersedes != "" {
		supersedes, err := id.Parse(body.Supersedes)
		if err != nil {
			return command, invalid("supersedes_instance_id", "supersedes_instance_id должен быть UUID.")
		}
		command.Supersedes = &supersedes
	}
	for _, approval := range body.ApprovedBy {
		participant, err := id.Parse(approval.ParticipantID)
		if err != nil {
			return command, invalid("approved_by", "participant_id каждого approver — UUID участника Tenant.")
		}
		command.ApprovedBy = append(command.ApprovedBy, employeeDomain.GateApproval{ParticipantID: participant, Role: approval.Role})
	}
	command.EvidenceRefs = body.EvidenceRefs
	command.EvidenceHash = body.EvidenceHash
	corr, _ := correlation.FromContext(r.Context())
	command.CorrelationID = corr
	return command, nil
}

func (s *Server) readinessViewAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.ReadinessView(r.Context(), actor, tenant, employee, r.URL.Query().Get("stage"))
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) gateInstancesViewAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.GateInstancesView(r.Context(), actor, tenant, employee)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) gateTransitionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	var body gateTransitionBody
	if err := decodeReadinessBody(w, r, &body); err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	command, err := gateTransitionCommand(s, r, actor, tenant, employee, body)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	instance, err := service.RecordGateTransition(r.Context(), command)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(instance)
}

func (s *Server) admissionHistoryAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.AdmissionHistoryView(r.Context(), actor, tenant, employee)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) publishAdmissionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	var body struct {
		Stage string `json:"stage"`
	}
	if err := decodeReadinessBody(w, r, &body); err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	admission, err := service.PublishAdmission(r.Context(), employeeAPI.PublishAdmissionCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr, Stage: body.Stage,
	})
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(admission)
}

// readinessFormTarget is the shared redirect base of the panel forms.
func readinessFormTarget(tenant, employee id.UUID) string {
	return "/app/tenants/" + tenant.String() + "/employees/" + employee.String() + "/test-release"
}

// redirectReadinessError flashes the honest message and returns to the
// panel (aria-live region announces it after the redirect).
func redirectReadinessError(w http.ResponseWriter, r *http.Request, tenant, employee id.UUID, err error) {
	var rule employeeDomain.ProfileRuleError
	var admissionConflict employeeDomain.AdmissionConflictError
	message := "Действие не выполнено. Повторите позднее."
	switch {
	case errors.Is(err, employeeDomain.ErrReadinessStepUpRequired):
		message = "Команда требует step-up подтверждения: провайдер повторной аутентификации не подключён в этом срезе — отказ честный, действие не выполняется."
	case errors.As(err, &admissionConflict):
		message = "Решение этой стадии этого кандидата уже действует (решение №" + strconv.FormatInt(decisionNoOf(admissionConflict.Current), 10) + "): ровно одна запись, повторная публикация не создаётся."
	case errors.Is(err, employeeDomain.ErrReadinessForbidden):
		message = "Команды Test & Release требуют полномочия configuration."
	case errors.As(err, &rule):
		message = rule.Reason
		if len(rule.Problems) > 1 {
			message += " Всего незакрытых пунктов: " + strconv.Itoa(len(rule.Problems)) + "."
		}
	case errors.Is(err, employeeDomain.ErrNotFound):
		message = "Сотрудник, профиль или манифест не найдены."
	}
	setFlashError(w, tenant, message)
	http.Redirect(w, r, readinessFormTarget(tenant, employee), http.StatusSeeOther)
}

func decisionNoOf(admission *employeeDomain.ReleaseAdmission) int64 {
	if admission == nil {
		return 0
	}
	return admission.DecisionNo
}

func (s *Server) testReleasePage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.ReadinessView(r.Context(), actor, tenant, employee, r.URL.Query().Get("stage"))
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	gates, err := service.GateInstancesView(r.Context(), actor, tenant, employee)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	data := map[string]any{
		"TenantID":   tenant,
		"EmployeeID": employee,
		"View":       prepareReadinessView(view),
		"Gates":      prepareGateInstancesView(gates),
	}
	for _, flag := range []struct {
		query  string
		notice string
	}{
		{"published", "Публикация записана: append-only решение с точными digest'ами, per-route результатами и approver-ролями."},
		{"gate_recorded", "Переход gate-инстанса записан: новая запись цепочки supersedes, история видна."},
	} {
		if v := r.URL.Query().Get(flag.query); v == "1" {
			data["Notice"] = flag.notice
		}
	}
	if message := flashError(r, tenant); message != "" {
		data["Error"] = message
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	s.render(w, "test-release", data)
}

func (s *Server) gateTransitionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		redirectReadinessError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "form", Field: "form", Reason: "Проверьте данные формы.", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	command := employeeAPI.GateTransitionCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee,
		DefinitionID: r.FormValue("definition_id"), Modality: r.FormValue("modality"),
		TargetState: r.FormValue("state"), NotApplicableReason: r.FormValue("not_applicable_reason"),
		ExpiryTrigger: r.FormValue("expiry_trigger"),
	}
	if channel := r.FormValue("route_channel"); channel != "" {
		command.Route = &employeeDomain.Route{Channel: channel, Direction: r.FormValue("route_direction")}
	}
	if supersedes := strings.TrimSpace(r.FormValue("supersedes_instance_id")); supersedes != "" {
		if parsed, err := id.Parse(supersedes); err == nil {
			command.Supersedes = &parsed
		} else {
			redirectReadinessError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "gate_instance", Field: "supersedes_instance_id", Reason: "supersedes_instance_id должен быть UUID.", Kind: employeeDomain.ProfileProblemInvalid})
			return
		}
	}
	for _, line := range splitLines(r.FormValue("approved_by")) {
		parts := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(parts) != 2 {
			redirectReadinessError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "gate_instance", Field: "approved_by", Reason: "Каждая строка approved_by: <UUID участника> <роль> (роль — метка реестра определения).", Kind: employeeDomain.ProfileProblemInvalid})
			return
		}
		participant, err := id.Parse(strings.TrimSpace(parts[0]))
		if err != nil {
			redirectReadinessError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "gate_instance", Field: "approved_by", Reason: "participant_id каждого approver — UUID участника Tenant.", Kind: employeeDomain.ProfileProblemInvalid})
			return
		}
		command.ApprovedBy = append(command.ApprovedBy, employeeDomain.GateApproval{ParticipantID: participant, Role: strings.TrimSpace(parts[1])})
	}
	command.EvidenceRefs = splitLines(r.FormValue("evidence_refs"))
	command.EvidenceHash = strings.TrimSpace(r.FormValue("evidence_hash"))
	corr, _ := correlation.FromContext(r.Context())
	command.CorrelationID = corr
	if _, err := service.RecordGateTransition(r.Context(), command); err != nil {
		redirectReadinessError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, readinessFormTarget(tenant, employee)+"?gate_recorded=1", http.StatusSeeOther)
}

func (s *Server) publishAdmissionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.readinessScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		redirectReadinessError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "form", Field: "form", Reason: "Проверьте данные формы.", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	if _, err := service.PublishAdmission(r.Context(), employeeAPI.PublishAdmissionCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr, Stage: r.FormValue("stage"),
	}); err != nil {
		redirectReadinessError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, readinessFormTarget(tenant, employee)+"?published=1", http.StatusSeeOther)
}
