package httptransport

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
)

func (s *Server) employeeScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return actor, "", false
	}
	return actor, tenant, true
}
func (s *Server) employeeProblem(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := 500, "internal_error", "Не удалось выполнить действие. Повторите позднее."
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrForbidden):
		status, code, message = 404, "not_found", "Ресурс не найден."
	case errors.Is(err, domain.ErrInvalid):
		status, code, message = 400, "invalid_brief", "Проверьте поля Brief и версию."
	case errors.Is(err, domain.ErrConflict):
		status, code, message = 409, "version_conflict", "Черновик изменён другим участником. Обновите страницу и повторите действие."
	case errors.Is(err, domain.ErrIdempotency):
		status, code, message = 409, "idempotency_conflict", "Этот ключ уже использован для другого запроса."
	}
	s.problem(w, r, status, code, message, status >= 500, nil)
}
func decodeEmployee(w http.ResponseWriter, r *http.Request, out any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return domain.ErrInvalid
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	d.DisallowUnknownFields()
	if err = d.Decode(out); err != nil {
		return domain.ErrInvalid
	}
	if err = d.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.ErrInvalid
	}
	return nil
}
func (s *Server) createEmployeeAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeEmployee(w, r, &input); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	result, err := s.employee.Create(r.Context(), api.CreateCommand{Actor: actor, TenantID: tenant, Name: input.Name, IdempotencyKey: r.Header.Get("Idempotency-Key"), CorrelationID: corr})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/v1/tenants/"+tenant.String()+"/employees/"+result.Draft.EmployeeID.String())
	if result.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"employee": result.Draft, "launch_plan": domain.LaunchPlan(result.Draft)})
}
func (s *Server) listEmployeesAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	items, err := s.employee.List(r.Context(), actor, tenant)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"employees": items})
}
func (s *Server) readEmployeeAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	item, err := s.employee.Read(r.Context(), actor, tenant, employeeID)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"employee": item, "launch_plan": domain.LaunchPlan(item)})
}
func (s *Server) saveBriefAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	var input struct {
		Version int64        `json:"version"`
		Brief   domain.Brief `json:"brief"`
	}
	if err = decodeEmployee(w, r, &input); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	item, err := s.employee.Save(r.Context(), api.SaveCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: input.Version, Brief: input.Brief})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"employee": item, "launch_plan": domain.LaunchPlan(item)})
}
func (s *Server) employeesPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	items, err := s.employee.List(r.Context(), actor, tenant)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	s.render(w, "employees", map[string]any{"TenantID": tenant, "Employees": items, "Key": s.freshKey("")})
}
func (s *Server) createEmployeeForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	if !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	result, err := s.employee.Create(r.Context(), api.CreateCommand{Actor: actor, TenantID: tenant, Name: r.FormValue("name"), IdempotencyKey: r.FormValue("idempotency_key"), CorrelationID: corr})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+result.Draft.EmployeeID.String(), http.StatusSeeOther)
}
func (s *Server) employeePage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	item, err := s.employee.Read(r.Context(), actor, tenant, employeeID)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	s.render(w, "employee", map[string]any{"TenantID": tenant, "Employee": item, "Plan": domain.LaunchPlan(item), "Saved": r.URL.Query().Get("saved") == "1"})
}
func (s *Server) saveBriefForm(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	if !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err = r.ParseForm(); err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	brief := domain.Brief{Purpose: r.FormValue("purpose"), Name: r.FormValue("name"), Languages: splitLines(r.FormValue("languages")), TimeZone: r.FormValue("time_zone"), WorkingHours: domain.WorkingHours{Days: r.Form["days"], Start: r.FormValue("start"), End: r.FormValue("end")}, DesiredChannels: r.Form["desired_channels"], PrimaryTasks: splitLines(r.FormValue("primary_tasks"))}
	if r.FormValue("resolved_from_version") != "" && r.FormValue("confirm_overwrite") != "on" {
		s.problem(w, r, http.StatusBadRequest, "confirmation_required", "Сначала сравните версии и подтвердите замену сохранённого Brief.", false, nil)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	_, err = s.employee.Save(r.Context(), api.SaveCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: version, Brief: brief})
	if err != nil {
		if !errors.Is(err, domain.ErrInvalid) && !errors.Is(err, domain.ErrConflict) {
			s.employeeProblem(w, r, err)
			return
		}
		status := 400
		data := map[string]any{"TenantID": tenant, "Employee": domain.Draft{EmployeeID: employeeID, Version: version, Brief: brief}, "Plan": domain.LaunchPlan(domain.Draft{}), "Error": employeeMessage(err)}
		if errors.Is(err, domain.ErrConflict) {
			status = 409
			current, readErr := s.employee.Read(r.Context(), actor, tenant, employeeID)
			if readErr != nil {
				s.employeeProblem(w, r, readErr)
				return
			}
			data["Current"] = current
			data["Plan"] = domain.LaunchPlan(current)
			data["Employee"] = domain.Draft{EmployeeID: employeeID, OwnerID: current.OwnerID, Version: current.Version, Brief: brief}
			data["ResolvedFromVersion"] = version
		}
		s.renderStatus(w, status, "employee", data)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employeeID.String()+"?saved=1", http.StatusSeeOther)
}
func employeeMessage(err error) string {
	if errors.Is(err, domain.ErrConflict) {
		return "Черновик изменён другим участником. Сравните сохранённые и ваши значения; подтверждение ниже заменит сохранённый Brief вашей версией."
	}
	return "Проверьте обязательные поля и повторите сохранение."
}
func splitLines(value string) []string {
	parts := strings.Split(value, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
