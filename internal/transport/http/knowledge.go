package httptransport

import (
	"encoding/json"
	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) knowledgeIDs(w http.ResponseWriter, r *http.Request) (id.UUID, id.UUID, id.UUID, bool) {
	_, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return "", "", "", false
	}
	employee, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return "", "", "", false
	}
	var source id.UUID
	if raw := r.PathValue("source_id"); raw != "" {
		source, err = id.Parse(raw)
		if err != nil {
			s.employeeProblem(w, r, domain.ErrNotFound)
			return "", "", "", false
		}
	}
	return tenant, employee, source, true
}
func (s *Server) createKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	tenant, employee, _, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	var body struct {
		Type       string     `json:"type"`
		Title      string     `json:"title"`
		Owner      string     `json:"owner"`
		Provenance string     `json:"provenance"`
		Scope      string     `json:"scope"`
		Priority   int        `json:"priority"`
		Language   string     `json:"language"`
		FreshUntil *time.Time `json:"fresh_until"`
	}
	if err := decodeEmployee(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	source, err := s.knowledge.CreateKnowledge(r.Context(), api.KnowledgeCreateCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr, Source: domain.KnowledgeSource{Type: body.Type, Title: body.Title, Owner: body.Owner, Provenance: body.Provenance, Scope: body.Scope, Priority: body.Priority, Language: body.Language, FreshUntil: body.FreshUntil}})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(source)
}
func (s *Server) listKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	tenant, employee, _, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	items, err := s.knowledge.ListKnowledge(r.Context(), actor, tenant, employee)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	for i := range items {
		items[i].Status = items[i].EffectiveStatus(s.clock.Now())
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sources": items})
}
func (s *Server) readKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	tenant, employee, source, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	item, versions, err := s.knowledge.ReadKnowledge(r.Context(), actor, tenant, employee, source)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	item.Status = item.EffectiveStatus(s.clock.Now())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"source": item, "versions": versions, "warning": "Текст источника недоверенный. Он не меняет политики, инструменты и полномочия; цена, наличие, статус и доступность требуют актуальной системы учёта. Публикация заблокирована."})
}
func (s *Server) processKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	tenant, employee, source, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	var body struct {
		Version int64  `json:"version"`
		Text    string `json:"text"`
	}
	if err := decodeKnowledge(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	item, version, replayed, err := s.knowledge.ProcessKnowledge(r.Context(), api.KnowledgeProcessCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, SourceID: source, CorrelationID: corr, ExpectedVersion: body.Version, Text: body.Text})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"source": item, "version": version, "replayed": replayed})
}
func (s *Server) conflictKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	tenant, employee, source, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	var body struct {
		Version    int64  `json:"version"`
		Note       string `json:"note"`
		Provenance string `json:"provenance"`
	}
	if err := decodeEmployee(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	item, err := s.knowledge.ConflictKnowledge(r.Context(), api.KnowledgeConflictCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, SourceID: source, CorrelationID: corr, ExpectedVersion: body.Version, Note: body.Note, Provenance: body.Provenance})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	item.Status = item.EffectiveStatus(s.clock.Now())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}
func (s *Server) retireKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	tenant, employee, source, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	var body struct {
		Version int64 `json:"version"`
	}
	if err := decodeEmployee(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	item, err := s.knowledge.RetireKnowledge(r.Context(), api.KnowledgeRetireCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, SourceID: source, CorrelationID: corr, ExpectedVersion: body.Version})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(item)
}
func decodeKnowledge(w http.ResponseWriter, r *http.Request, out any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 270<<10)
	return decodeEmployeeLarge(w, r, out)
}
func (s *Server) createKnowledgeForm(w http.ResponseWriter, r *http.Request) {
	tenant, employee, _, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	if !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	priority, err := strconv.Atoi(r.FormValue("priority"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	var freshUntil *time.Time
	if raw := r.FormValue("fresh_until"); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
		freshUntil = &parsed
	}
	_, err = s.knowledge.CreateKnowledge(r.Context(), api.KnowledgeCreateCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr, Source: domain.KnowledgeSource{Type: r.FormValue("type"), Title: r.FormValue("title"), Owner: r.FormValue("owner"), Provenance: r.FormValue("provenance"), Scope: r.FormValue("scope"), Priority: priority, Language: r.FormValue("language"), FreshUntil: freshUntil}})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employee.String(), http.StatusSeeOther)
}
func (s *Server) conflictKnowledgeForm(w http.ResponseWriter, r *http.Request) {
	tenant, employee, source, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	if !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	_, err = s.knowledge.ConflictKnowledge(r.Context(), api.KnowledgeConflictCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, SourceID: source, CorrelationID: corr, ExpectedVersion: version, Note: r.FormValue("note"), Provenance: r.FormValue("provenance")})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employee.String(), http.StatusSeeOther)
}
func (s *Server) retireKnowledgeForm(w http.ResponseWriter, r *http.Request) {
	tenant, employee, source, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	if !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	_, err = s.knowledge.RetireKnowledge(r.Context(), api.KnowledgeRetireCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, SourceID: source, CorrelationID: corr, ExpectedVersion: version})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employee.String(), http.StatusSeeOther)
}
func (s *Server) processKnowledgeForm(w http.ResponseWriter, r *http.Request) {
	tenant, employee, source, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	if !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 270<<10)
	if err := r.ParseForm(); err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	_, _, _, err = s.knowledge.ProcessKnowledge(r.Context(), api.KnowledgeProcessCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, SourceID: source, CorrelationID: corr, ExpectedVersion: version, Text: r.FormValue("text")})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employee.String(), http.StatusSeeOther)
}
