package httptransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/staffy-co/staffy/internal/employee/api"
	"github.com/staffy-co/staffy/internal/employee/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/correlation"
	"github.com/staffy-co/staffy/internal/platform/id"
)

func (s *Server) releaseService() api.ReleaseService { return s.release }

// releaseScope resolves the principal plus the tenant and employee path
// values of the release surface.
func (s *Server) releaseScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return actor, "", "", false
	}
	employee, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return actor, "", "", false
	}
	return actor, tenant, employee, true
}

// releaseProblem maps the release errors to honest problem responses. The
// optimistic-concurrency loss carries the expected and current revision so
// the caller can compare instead of blindly retrying.
func (s *Server) releaseProblem(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
	fields := map[string]string{}
	var rule domain.ReleaseRuleError
	var conflict domain.ReleaseConflictError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status, code, message = 404, "not_found", "Ресурс не найден."
	case errors.Is(err, domain.ErrReleaseForbidden):
		status, code, message = 403, "release_forbidden", "Версии конфигурации и манифесты доступны с полномочиями configuration, review или audit; управление — только configuration."
	case errors.As(err, &conflict):
		status, code, message = 409, "release_revision_conflict", "Черновик релиза изменился другим действием: ожидаемая ревизия не совпала с текущей. Сверьте текущее состояние и повторите с актуальной ревизией."
		fields["expected_revision"] = strconv.FormatInt(conflict.Expected, 10)
		fields["current_revision"] = strconv.FormatInt(conflict.Current, 10)
		fields["current_refs"] = releaseRefsSummary(conflict.Refs)
		fields["release_view"] = "GET /v1/tenants/{tenant_id}/employees/{employee_id}/release"
	case errors.Is(err, domain.ErrManifestExists):
		status, code, message = 409, "manifest_exists", "Манифест этой конфигурационной версии уже запечатан: под одной версией существует не более одного манифеста."
	case errors.As(err, &rule):
		status, code, message = 400, "release_rule", rule.Reason
		fields[rule.Rule] = rule.Reason
	case errors.Is(err, domain.ErrConflict):
		status, code, message = 409, "release_revision_conflict", "Черновик релиза изменён другим действием. Обновите страницу и повторите."
	case errors.Is(err, domain.ErrInvalid):
		status, code, message = 400, "invalid_release_command", "Проверьте параметры команды релиза."
	}
	s.problem(w, r, status, code, message, status >= 500, fields)
}

// BindRelease mounts the release surface: the JSON API and the accessible
// versions-and-manifests page (Story 1-12).
func (s *Server) BindRelease(service api.ReleaseService) {
	s.release = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/release", s.releaseViewAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/release/draft", s.assembleDraftAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/release/versions", s.createVersionAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/release/manifests", s.sealManifestAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/employees/{employee_id}/release/manifests/{config_version}", s.readManifestAPI)
	s.mux.HandleFunc("POST /v1/tenants/{tenant_id}/employees/{employee_id}/release/restore", s.restoreAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/employees/{employee_id}/release", s.releasePage)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/release/draft", s.assembleDraftForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/release/versions", s.createVersionForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/release/manifests", s.sealManifestForm)
	s.mux.HandleFunc("POST /app/tenants/{tenant_id}/employees/{employee_id}/release/restore", s.restoreForm)
}

// decodeReleaseBody reads one strict JSON command body.
func decodeReleaseBody(w http.ResponseWriter, r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return domain.ReleaseRuleError{Rule: "content_type", Reason: "Ожидается application/json."}
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return domain.ReleaseRuleError{Rule: "body", Reason: "Тело команды не распознано: проверьте поля и типы."}
	}
	if err = d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.ReleaseRuleError{Rule: "body", Reason: "Тело команды содержит лишние данные."}
	}
	return nil
}

func (s *Server) releaseViewAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	view, err := service.ReleaseView(r.Context(), actor, tenant, employee)
	if err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) readManifestAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	version, err := strconv.ParseInt(r.PathValue("config_version"), 10, 64)
	if err != nil || version < 1 {
		s.releaseProblem(w, r, domain.ReleaseRuleError{Rule: "config_version", Reason: "Номер конфигурационной версии указан неверно."})
		return
	}
	view, err := service.ReadReleaseManifest(r.Context(), actor, tenant, employee, version)
	if err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (s *Server) assembleDraftAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	var body struct {
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if err := decodeReleaseBody(w, r, &body); err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	draft, err := service.AssembleDraft(r.Context(), api.AssembleCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr, ExpectedRevision: body.ExpectedRevision,
	})
	if err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(draft)
}

func (s *Server) createVersionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	var body struct {
		Description      string `json:"description"`
		ExpectedRevision int64  `json:"expected_revision"`
	}
	if err := decodeReleaseBody(w, r, &body); err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	version, err := service.CreateConfigVersion(r.Context(), api.VersionCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		Description: body.Description, ExpectedRevision: body.ExpectedRevision,
	})
	if err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(version)
}

func (s *Server) sealManifestAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	var body struct {
		ConfigVersion    int64 `json:"config_version"`
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if err := decodeReleaseBody(w, r, &body); err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	manifest, err := service.SealManifest(r.Context(), api.SealCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		ConfigVersion: body.ConfigVersion, ExpectedRevision: body.ExpectedRevision,
	})
	if err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(manifest)
}

func (s *Server) restoreAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	var body struct {
		ConfigVersion    int64 `json:"config_version"`
		ExpectedRevision int64 `json:"expected_revision"`
	}
	if err := decodeReleaseBody(w, r, &body); err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	result, err := service.Restore(r.Context(), api.RestoreCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		ConfigVersion: body.ConfigVersion, ExpectedRevision: body.ExpectedRevision,
	})
	if err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// releaseRefsSummary renders the current draft refs as a compact textual
// comparison basis for the conflict answer (the spec requires the current
// refs to travel with the honest 409).
func releaseRefsSummary(refs domain.ConfigRefs) string {
	return fmt.Sprintf("brief=%s(%d) domain_pack=%s behavior=%s knowledge=%s voice=%s",
		refs.Brief.Status, refs.Brief.DraftVersion,
		releaseRefValue(refs.Pack.Status, refs.Pack.ID, refs.Pack.Version),
		releaseRefValue(refs.Behavior.Status, "", strconv.FormatInt(refs.Behavior.Version, 10)),
		releaseRefValue(refs.Knowledge.Status, strconv.Itoa(len(refs.Knowledge.Sources)), ""),
		releaseRefValue(refs.Voice.Status, "", strconv.FormatInt(refs.Voice.Version, 10)))
}

func releaseRefValue(status string, left, right string) string {
	if status != domain.RefPresent {
		return status
	}
	switch {
	case left != "" && right != "":
		return left + "@" + right
	case left != "":
		return left
	default:
		return right
	}
}

// releaseFormTarget is the shared redirect base of the release forms.
func releaseFormTarget(tenant, employee id.UUID) string {
	return "/app/tenants/" + tenant.String() + "/employees/" + employee.String() + "/release"
}

// redirectReleaseError flashes the honest message and returns to the page.
func redirectReleaseError(w http.ResponseWriter, r *http.Request, tenant, employee id.UUID, err error) {
	var rule domain.ReleaseRuleError
	var conflict domain.ReleaseConflictError
	message := "Действие не выполнено. Повторите позднее."
	switch {
	case errors.As(err, &conflict):
		message = "Конфликт: ожидаемая ревизия черновика " + strconv.FormatInt(conflict.Expected, 10) +
			", текущая " + strconv.FormatInt(conflict.Current, 10) + ". Страница показывает актуальное состояние — сверьте и повторите."
	case errors.Is(err, domain.ErrManifestExists):
		message = "Манифест этой версии уже запечатан: второй манифест не создаётся."
	case errors.Is(err, domain.ErrReleaseForbidden):
		message = "Управление релизом требует полномочия configuration."
	case errors.As(err, &rule):
		message = rule.Reason
	case errors.Is(err, domain.ErrNotFound):
		message = "Сотрудник или версия не найдены."
	}
	setFlashError(w, tenant, message)
	http.Redirect(w, r, releaseFormTarget(tenant, employee), http.StatusSeeOther)
}

func (s *Server) releasePage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	view, err := service.ReleaseView(r.Context(), actor, tenant, employee)
	if err != nil {
		s.releaseProblem(w, r, err)
		return
	}
	data := map[string]any{
		"TenantID":   tenant,
		"EmployeeID": employee,
		"View":       prepareReleaseView(view),
	}
	for _, flag := range []struct {
		query  string
		notice string
	}{
		{"draft_saved", "Черновик конфигурации собран из текущих фактов."},
		{"version_created", "Конфигурационная версия создана и неизменяема."},
		{"manifest_sealed", "Манифест запечатан: снимок и digest зафиксированы."},
		{"restored", "Черновик восстановлен из выбранной версии: сверьте отчёт о несовместимостях."},
	} {
		if v := r.URL.Query().Get(flag.query); v == "1" {
			data["Notice"] = flag.notice
		}
	}
	if message := flashError(r, tenant); message != "" {
		data["Error"] = message
		http.SetCookie(w, clearFlashCookie(tenant))
	}
	s.render(w, "release", data)
}

func (s *Server) assembleDraftForm(w http.ResponseWriter, r *http.Request) {
	s.releaseCommandForm(w, r, func(actor identityDomain.Principal, tenant, employee id.UUID, revision int64, corr id.UUID) (string, error) {
		service := s.releaseService()
		if service == nil {
			return "", domain.ErrNotFound
		}
		if _, err := service.AssembleDraft(r.Context(), api.AssembleCommand{
			Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr, ExpectedRevision: revision,
		}); err != nil {
			return "", err
		}
		return "draft_saved", nil
	})
}

func (s *Server) createVersionForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.releaseProblem(w, r, domain.ReleaseRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	revision, err := formRevision(r)
	if err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	if _, err := service.CreateConfigVersion(r.Context(), api.VersionCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		Description: r.FormValue("description"), ExpectedRevision: revision,
	}); err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, releaseFormTarget(tenant, employee)+"?version_created=1", http.StatusSeeOther)
}

func (s *Server) sealManifestForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.releaseProblem(w, r, domain.ReleaseRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	revision, err := formRevision(r)
	if err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	version, err := strconv.ParseInt(r.FormValue("config_version"), 10, 64)
	if err != nil || version < 1 {
		redirectReleaseError(w, r, tenant, employee, domain.ReleaseRuleError{Rule: "config_version", Reason: "Номер конфигурационной версии указан неверно."})
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	if _, err := service.SealManifest(r.Context(), api.SealCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		ConfigVersion: version, ExpectedRevision: revision,
	}); err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, releaseFormTarget(tenant, employee)+"?manifest_sealed=1", http.StatusSeeOther)
}

func (s *Server) restoreForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	service := s.releaseService()
	if service == nil {
		s.releaseProblem(w, r, domain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.releaseProblem(w, r, domain.ReleaseRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	revision, err := formRevision(r)
	if err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	version, err := strconv.ParseInt(r.FormValue("config_version"), 10, 64)
	if err != nil || version < 1 {
		redirectReleaseError(w, r, tenant, employee, domain.ReleaseRuleError{Rule: "config_version", Reason: "Номер конфигурационной версии указан неверно."})
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	if _, err := service.Restore(r.Context(), api.RestoreCommand{
		Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr,
		ConfigVersion: version, ExpectedRevision: revision,
	}); err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, releaseFormTarget(tenant, employee)+"?restored=1", http.StatusSeeOther)
}

// releaseCommandForm is the shared shape of the assemble form (revision only).
func (s *Server) releaseCommandForm(w http.ResponseWriter, r *http.Request, run func(identityDomain.Principal, id.UUID, id.UUID, int64, id.UUID) (string, error)) {
	actor, tenant, employee, ok := s.releaseScope(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.releaseProblem(w, r, domain.ReleaseRuleError{Rule: "form", Reason: "Проверьте данные формы."})
		return
	}
	revision, err := formRevision(r)
	if err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	flag, err := run(actor, tenant, employee, revision, corr)
	if err != nil {
		redirectReleaseError(w, r, tenant, employee, err)
		return
	}
	http.Redirect(w, r, releaseFormTarget(tenant, employee)+"?"+flag+"=1", http.StatusSeeOther)
}

// formRevision parses the optimistic-concurrency revision of a release form.
func formRevision(r *http.Request) (int64, error) {
	value := r.FormValue("expected_revision")
	if value == "" {
		return 0, nil
	}
	revision, err := strconv.ParseInt(value, 10, 64)
	if err != nil || revision < 0 {
		return 0, domain.ReleaseRuleError{Rule: "expected_revision", Reason: "Ожидаемая ревизия указана неверно."}
	}
	return revision, nil
}
