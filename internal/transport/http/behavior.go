package httptransport

import (
	"encoding/json"
	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Server) behaviorCommand(w http.ResponseWriter, r *http.Request) (api.BehaviorCommand, bool) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return api.BehaviorCommand{}, false
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return api.BehaviorCommand{}, false
	}
	var body struct {
		Version       int64           `json:"version"`
		Config        domain.Behavior `json:"config"`
		ChangeSummary string          `json:"change_summary"`
		Scenario      string          `json:"scenario"`
		PackID        string          `json:"pack_id"`
		PackVersion   string          `json:"pack_version"`
	}
	if err = decodeEmployeeLarge(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return api.BehaviorCommand{}, false
	}
	corr, _ := correlation.FromContext(r.Context())
	return api.BehaviorCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: body.Version, Config: body.Config, ChangeSummary: body.ChangeSummary, Scenario: body.Scenario, PackID: body.PackID, PackVersion: body.PackVersion}, true
}
func (s *Server) behaviorService() api.BehaviorService {
	behavior, _ := s.employee.(api.BehaviorService)
	return behavior
}
func (s *Server) readBehaviorAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	behavior := s.behaviorService()
	if behavior == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	versions, err := behavior.ReadBehavior(r.Context(), actor, tenant, employeeID)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"versions": versions})
}
func (s *Server) saveBehaviorAPI(w http.ResponseWriter, r *http.Request) {
	c, ok := s.behaviorCommand(w, r)
	if !ok {
		return
	}
	behavior := s.behaviorService()
	if behavior == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	v, err := behavior.SaveBehavior(r.Context(), c)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) previewBehaviorAPI(w http.ResponseWriter, r *http.Request) {
	c, ok := s.behaviorCommand(w, r)
	if !ok {
		return
	}
	behavior := s.behaviorService()
	if behavior == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	v, err := behavior.PreviewBehavior(r.Context(), c)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) saveBehaviorForm(w http.ResponseWriter, r *http.Request) {
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
	config := domain.Behavior{Role: r.FormValue("role"), Purpose: r.FormValue("purpose"), AIDisclosure: r.FormValue("ai_disclosure") == "on", Tone: r.FormValue("tone"), Modes: r.Form["modes"], Confirmation: r.FormValue("confirmation"), Handoff: r.FormValue("handoff"), ForbiddenPhrases: splitLines(r.FormValue("forbidden_phrases")), Examples: splitLines(r.FormValue("examples"))}
	if text := strings.TrimSpace(r.FormValue("claim_text")); text != "" {
		var sourceID id.UUID
		var sourceVersion int64
		var reviewAt time.Time
		if value := r.FormValue("claim_evidence_source_id"); value != "" {
			sourceID, err = id.Parse(value)
			if err != nil {
				s.employeeProblem(w, r, domain.ErrInvalid)
				return
			}
		}
		if value := r.FormValue("claim_evidence_version"); value != "" {
			sourceVersion, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				s.employeeProblem(w, r, domain.ErrInvalid)
				return
			}
		}
		if value := r.FormValue("claim_review_at"); value != "" {
			reviewAt, err = time.Parse(time.RFC3339, value)
			if err != nil {
				s.employeeProblem(w, r, domain.ErrInvalid)
				return
			}
		}
		config.Claims = []domain.AllowedClaim{{Text: text, Owner: r.FormValue("claim_owner"), Scope: r.FormValue("claim_scope"), Channels: r.Form["claim_channels"], Modes: r.Form["claim_modes"], EvidenceSourceID: sourceID, EvidenceVersion: sourceVersion, ReviewAt: reviewAt}}
	}
	corr, _ := correlation.FromContext(r.Context())
	behavior := s.behaviorService()
	if behavior == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	versions, err := behavior.ReadBehavior(r.Context(), actor, tenant, employeeID)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	if len(versions) > 0 {
		newClaims := config.Claims
		config.Claims = nil
		for i, claim := range versions[0].Config.Claims {
			if r.FormValue("existing_claim_delete_"+strconv.Itoa(i)) == "on" {
				continue
			}
			claim.Text = r.FormValue("existing_claim_text_" + strconv.Itoa(i))
			claim.Owner = r.FormValue("existing_claim_owner_" + strconv.Itoa(i))
			claim.Scope = r.FormValue("existing_claim_scope_" + strconv.Itoa(i))
			config.Claims = append(config.Claims, claim)
		}
		config.Claims = append(config.Claims, newClaims...)
	}
	_, err = behavior.SaveBehavior(r.Context(), api.BehaviorCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: version, Config: config, ChangeSummary: r.FormValue("change_summary"), PackID: r.FormValue("pack_id"), PackVersion: r.FormValue("pack_version")})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employeeID.String()+"?saved=1", http.StatusSeeOther)
}
