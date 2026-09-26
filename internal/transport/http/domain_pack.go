package httptransport

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/employee/packs"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
)

func (s *Server) domainCatalogAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	items, err := s.employee.Catalog(r.Context(), actor, tenant)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"domain_packs": items})
}

func (s *Server) packCommand(w http.ResponseWriter, r *http.Request) (api.PinCommand, bool) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return api.PinCommand{}, false
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return api.PinCommand{}, false
	}
	var body struct {
		Version     int64          `json:"version"`
		PackID      string         `json:"pack_id"`
		PackVersion string         `json:"pack_version"`
		Variants    map[string]any `json:"variants"`
		Confirmed   bool           `json:"confirmed"`
	}
	if err := decodeEmployee(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return api.PinCommand{}, false
	}
	corr, _ := correlation.FromContext(r.Context())
	return api.PinCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: body.Version, PackID: body.PackID, PackVersion: body.PackVersion, Variants: body.Variants, Confirmed: body.Confirmed}, true
}
func (s *Server) previewPackAPI(w http.ResponseWriter, r *http.Request) {
	c, ok := s.packCommand(w, r)
	if !ok {
		return
	}
	result, err := s.employee.Preview(r.Context(), c)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
func (s *Server) pinPackAPI(w http.ResponseWriter, r *http.Request) {
	c, ok := s.packCommand(w, r)
	if !ok {
		return
	}
	result, err := s.employee.Pin(r.Context(), c)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"employee": result, "launch_plan": domain.LaunchPlan(result)})
}

func (s *Server) pinPackForm(w http.ResponseWriter, r *http.Request) {
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
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err = r.ParseForm(); err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	choice := r.FormValue("pack_choice")
	separator := strings.LastIndexByte(choice, '@')
	if separator < 1 {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	packID, packVersion := choice[:separator], choice[separator+1:]
	manifest, available := packs.Get(packID, packVersion)
	if !available {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	variants := map[string]any{}
	for name, values := range r.Form {
		if !strings.HasPrefix(name, "variant.") {
			continue
		}
		key := strings.TrimPrefix(name, "variant.")
		field, known := manifest.Schema[key]
		if !known || field.Protected || len(values) != 1 {
			s.employeeProblem(w, r, domain.VariantError{Fields: map[string]string{key: "Поле нельзя изменять."}})
			return
		}
		value := values[0]
		switch field.Type {
		case "string":
			variants[key] = value
		case "boolean":
			if value != "true" && value != "false" {
				s.employeeProblem(w, r, domain.VariantError{Fields: map[string]string{key: "Ожидается да или нет."}})
				return
			}
			variants[key] = value == "true"
		case "integer":
			n, e := strconv.Atoi(value)
			if e != nil {
				s.employeeProblem(w, r, domain.VariantError{Fields: map[string]string{key: "Ожидается число."}})
				return
			}
			variants[key] = float64(n)
		default:
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
	}
	corr, _ := correlation.FromContext(r.Context())
	c := api.PinCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: version, PackID: packID, PackVersion: packVersion, Variants: variants, Confirmed: r.FormValue("confirmed") == "on"}
	if _, err = s.employee.Pin(r.Context(), c); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employeeID.String()+"?saved=1", http.StatusSeeOther)
}
