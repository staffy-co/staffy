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

func (s *Server) profileService() employeeAPI.ProfileService { return s.profile }

// profileScope resolves the principal plus the tenant and employee path
// values of the operating profile surface.
func (s *Server) profileScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return actor, "", "", false
	}
	employee, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return actor, "", "", false
	}
	return actor, tenant, employee, true
}

// profileProblem maps the profile errors to honest problem responses. The
// optimistic-concurrency loss carries the expected and current revision; a
// rule rejection carries every section.field of the honest list.
func (s *Server) profileProblem(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
	fields := map[string]string{}
	var rule employeeDomain.ProfileRuleError
	var conflict employeeDomain.ProfileConflictError
	switch {
	case errors.Is(err, employeeDomain.ErrNotFound):
		status, code, message = 404, "not_found", "Ресурс не найден."
	case errors.Is(err, employeeDomain.ErrProfileForbidden):
		status, code, message = 403, "profile_forbidden", "Operating Profile доступен с полномочиями configuration, review или audit; управление — только configuration."
	case errors.Is(err, employeeDomain.ErrProfileStepUpRequired):
		status, code, message = 403, "step_up_required", "Утверждение профиля требует step-up подтверждения: повторная аутентификация не подключена в этом срезе (честный отказ, не обход)."
	case errors.As(err, &conflict):
		status, code, message = 409, "profile_revision_conflict", "Черновик профиля изменился другим действием: ожидаемая ревизия не совпала с текущей. Сверьте текущее состояние и повторите с актуальной ревизией."
		fields["expected_revision"] = strconv.FormatInt(conflict.Expected, 10)
		fields["current_revision"] = strconv.FormatInt(conflict.Current, 10)
		fields["profile_view"] = "GET /v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile"
		// The spec requires the conflict to carry the current draft
		// sections for an honest comparison — the canonical JSON of
		// the current content travels with the refusal instead of a
		// blind retry.
		if conflict.CurrentContent != nil {
			if raw, jsonErr := employeeDomain.CanonicalProfileJSON(*conflict.CurrentContent); jsonErr == nil {
				fields["current_content"] = string(raw)
			}
		}
	case errors.As(err, &rule):
		status, code, message = 400, "profile_rule", rule.Reason
		fields["section"] = rule.Section
		fields["field"] = rule.Field
		if rule.Kind != "" {
			fields["kind"] = rule.Kind
		}
		for index, problem := range rule.Problems {
			fields["problem_"+strconv.Itoa(index)] = problem.Section + "." + problem.Field + " (" + problem.Kind + "): " + problem.Reason
		}
	case errors.Is(err, employeeDomain.ErrConflict):
		status, code, message = 409, "profile_revision_conflict", "Конкурентная команда уже применилась: обновите состояние и повторите."
	case errors.Is(err, employeeDomain.ErrInvalid):
		status, code, message = 400, "invalid_profile_command", "Проверьте параметры команды профиля."
	}
	s.problem(w, r, status, code, message, status >= 500, fields)
}

// BindProfile mounts the operating profile surface: the JSON API and the
// accessible profile page (Story 1-13).
func (s *Server) BindProfile(service employeeAPI.ProfileService) {
	s.profile = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile", s.profileViewAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/draft", s.updateDraftAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/approval", s.approveProfileAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds", s.decideThresholdAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds/{gate_id}", s.thresholdHistoryAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/employees/{employee_id}/operating-profile", s.profilePage)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/operating-profile/draft", s.updateDraftForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/operating-profile/approval", s.approveProfileForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/operating-profile/thresholds", s.decideThresholdForm)
}

// decodeProfileBody reads one strict JSON command body.
func decodeProfileBody(w http.ResponseWriter, r *http.Request, out any) error {
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

// profileDraftBody is the wire shape of the draft command: the exact
// manifest link plus the whole profile content.
type profileDraftBody struct {
	Manifest struct {
		ManifestID    string `json:"manifest_id"`
		ConfigVersion int64  `json:"config_version"`
		Digest        string `json:"digest"`
	} `json:"manifest"`
	Content          employeeDomain.ProfileContent `json:"content"`
	ExpectedRevision int64                         `json:"expected_revision"`
}

func (s *Server) profileViewAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.ProfileView(r.Context(), actor, tenant, employee)
	if err != nil {
		s.profileProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) updateDraftAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	var body profileDraftBody
	if err := decodeProfileBody(w, r, &body); err != nil {
		s.profileProblem(w, r, err)
		return
	}
	command, err := profileDraftCommand(s, r, actor, tenant, employee, body)
	if err != nil {
		s.profileProblem(w, r, err)
		return
	}
	draft, err := service.UpdateDraft(r.Context(), command)
	if err != nil {
		s.profileProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(draft)
}

// profileDraftCommand validates the wire values and builds the command.
func profileDraftCommand(s *Server, r *http.Request, actor identityDomain.Principal, tenant, employee id.UUID, body profileDraftBody) (employeeAPI.ProfileDraftCommand, error) {
	manifestID, err := id.Parse(body.Manifest.ManifestID)
	if err != nil {
		return employeeAPI.ProfileDraftCommand{}, employeeDomain.ProfileRuleError{Section: "manifest", Field: "manifest_id", Reason: "manifest_id должен быть UUID.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	if body.Manifest.ConfigVersion < 1 {
		return employeeAPI.ProfileDraftCommand{}, employeeDomain.ProfileRuleError{Section: "manifest", Field: "config_version", Reason: "config_version: целое число ≥ 1.", Kind: employeeDomain.ProfileProblemInvalid}
	}
	corr, _ := correlation.FromContext(r.Context())
	return employeeAPI.ProfileDraftCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		Manifest: employeeDomain.ManifestLink{ManifestID: manifestID, ConfigVersion: body.Manifest.ConfigVersion, Digest: body.Manifest.Digest},
		Content:  body.Content, ExpectedRevision: body.ExpectedRevision,
	}, nil
}

func (s *Server) approveProfileAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	var body struct {
		Description      string   `json:"description"`
		EvidenceRefs     []string `json:"evidence_refs"`
		ExpectedRevision int64    `json:"expected_revision"`
	}
	if err := decodeProfileBody(w, r, &body); err != nil {
		s.profileProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	version, err := service.ApproveProfile(r.Context(), employeeAPI.ProfileApproveCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		Description: body.Description, EvidenceRefs: body.EvidenceRefs, ExpectedRevision: body.ExpectedRevision,
	})
	if err != nil {
		s.profileProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(version)
}

func (s *Server) decideThresholdAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	var body struct {
		GateID       string   `json:"gate_id"`
		Value        string   `json:"value"`
		Decision     string   `json:"decision"`
		EvidenceRefs []string `json:"evidence_refs"`
	}
	if err := decodeProfileBody(w, r, &body); err != nil {
		s.profileProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	decision, err := service.DecideThreshold(r.Context(), employeeAPI.ThresholdCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		GateID: body.GateID, Value: body.Value, Decision: body.Decision, EvidenceRefs: body.EvidenceRefs,
	})
	if err != nil {
		s.profileProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(decision)
}

func (s *Server) thresholdHistoryAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	history, err := service.ThresholdHistory(r.Context(), actor, tenant, employee, r.PathValue("gate_id"))
	if err != nil {
		s.profileProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(history)
}

// profileFormTarget is the shared redirect base of the profile forms.
func profileFormTarget(tenant, employee id.UUID) string {
	return "/app/tenants/" + tenant.String() + "/employees/" + employee.String() + "/operating-profile"
}

// redirectProfileError flashes the honest message and returns to the page.
func redirectProfileError(w http.ResponseWriter, r *http.Request, tenant, employee id.UUID, err error) {
	var rule employeeDomain.ProfileRuleError
	var conflict employeeDomain.ProfileConflictError
	message := "Действие не выполнено. Повторите позднее."
	switch {
	case errors.Is(err, employeeDomain.ErrProfileStepUpRequired):
		message = "Утверждение требует step-up подтверждения: провайдер повторной аутентификации не подключён в этом срезе — отказ честный, утверждение не выполняется."
	case errors.As(err, &conflict):
		message = "Конфликт: ожидаемая ревизия черновика " + strconv.FormatInt(conflict.Expected, 10) +
			", текущая " + strconv.FormatInt(conflict.Current, 10) + ". Страница показывает актуальное состояние — сверьте и повторите."
	case errors.Is(err, employeeDomain.ErrProfileForbidden):
		message = "Управление профилем требует полномочия configuration."
	case errors.As(err, &rule):
		message = rule.Reason
		if len(rule.Problems) > 1 {
			message += " Всего незакрытых пунктов: " + strconv.Itoa(len(rule.Problems)) + "."
		}
	case errors.Is(err, employeeDomain.ErrNotFound):
		message = "Сотрудник, манифест или профиль не найдены."
	}
	setFlashError(w, tenant, message)
	http.Redirect(w, r, profileFormTarget(tenant, employee), http.StatusSeeOther)
}

func (s *Server) profilePage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.ProfileView(r.Context(), actor, tenant, employee)
	if err != nil {
		s.profileProblem(w, r, err)
		return
	}
	data := map[string]any{
		"TenantID":   tenant,
		"EmployeeID": employee,
		"View":       prepareProfileView(view),
	}
	for _, flag := range []struct {
		query  string
		notice string
	}{
		{"draft_saved", "Черновик профиля сохранён с точной ссылкой на манифест."},
		{"profile_approved", "Профиль утверждён: версия, digest и approver-роли зафиксированы неизменяемо."},
		{"threshold_decided", "Порог записан новой версией решения с историей."},
	} {
		if v := r.URL.Query().Get(flag.query); v == "1" {
			data["Notice"] = flag.notice
		}
	}
	if message := flashError(r, tenant); message != "" {
		data["Error"] = message
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	s.render(w, "operating-profile", data)
}

func (s *Server) updateDraftForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		redirectProfileError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "form", Field: "form", Reason: "Проверьте данные формы.", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	revision, err := formRevision(r)
	if err != nil {
		redirectProfileError(w, r, tenant, employee, err)
		return
	}
	manifestID, err := id.Parse(strings.TrimSpace(r.FormValue("manifest_id")))
	if err != nil {
		redirectProfileError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "manifest", Field: "manifest_id", Reason: "manifest_id запечатанного манифеста должен быть UUID.", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	configVersion, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("config_version")), 10, 64)
	if err != nil || configVersion < 1 {
		redirectProfileError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "manifest", Field: "config_version", Reason: "Номер конфигурационной версии указан неверно.", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	var content employeeDomain.ProfileContent
	raw := strings.TrimSpace(r.FormValue("content"))
	if raw == "" {
		content = employeeDomain.ProfileContent{}
	} else if err := decodeProfileJSON([]byte(raw), &content); err != nil {
		redirectProfileError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "content", Field: "json", Reason: "Содержание профиля не распознано как корректный JSON структуры профиля: " + err.Error() + ".", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	if _, err := service.UpdateDraft(r.Context(), employeeAPI.ProfileDraftCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		Manifest: employeeDomain.ManifestLink{ManifestID: manifestID, ConfigVersion: configVersion, Digest: strings.TrimSpace(r.FormValue("manifest_digest"))},
		Content:  content, ExpectedRevision: revision,
	}); err != nil {
		redirectProfileError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, profileFormTarget(tenant, employee)+"?draft_saved=1", http.StatusSeeOther)
}

// decodeProfileJSON decodes the content strictly: unknown fields are honest
// errors, not silent drops.
func decodeProfileJSON(raw []byte, out *employeeDomain.ProfileContent) error {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("лишние данные после JSON-объекта")
	}
	return nil
}

func (s *Server) approveProfileForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		redirectProfileError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "form", Field: "form", Reason: "Проверьте данные формы.", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	revision, err := formRevision(r)
	if err != nil {
		redirectProfileError(w, r, tenant, employee, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	if _, err := service.ApproveProfile(r.Context(), employeeAPI.ProfileApproveCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		Description: r.FormValue("description"), EvidenceRefs: splitLines(r.FormValue("evidence_refs")), ExpectedRevision: revision,
	}); err != nil {
		redirectProfileError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, profileFormTarget(tenant, employee)+"?profile_approved=1", http.StatusSeeOther)
}

func (s *Server) decideThresholdForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.profileScope(w, r)
	if !ok {
		return
	}
	service := s.profileService()
	if service == nil {
		s.profileProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		redirectProfileError(w, r, tenant, employee, employeeDomain.ProfileRuleError{Section: "form", Field: "form", Reason: "Проверьте данные формы.", Kind: employeeDomain.ProfileProblemInvalid})
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	if _, err := service.DecideThreshold(r.Context(), employeeAPI.ThresholdCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		GateID: r.FormValue("gate_id"), Value: r.FormValue("value"), Decision: r.FormValue("decision"),
		EvidenceRefs: splitLines(r.FormValue("evidence_refs")),
	}); err != nil {
		redirectProfileError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, profileFormTarget(tenant, employee)+"?threshold_decided=1", http.StatusSeeOther)
}
