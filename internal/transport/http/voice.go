package httptransport

import (
	"encoding/json"
	"errors"
	"github.com/artemiosu/staffy/internal/employee/api"
	"github.com/artemiosu/staffy/internal/employee/domain"
	"github.com/artemiosu/staffy/internal/platform/correlation"
	"github.com/artemiosu/staffy/internal/platform/id"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) voiceService() api.VoiceService { v, _ := s.employee.(api.VoiceService); return v }
func (s *Server) voiceCatalogAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	v := s.voiceService()
	if v == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	p, err := v.VoiceCatalog(r.Context(), actor, tenant)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"profiles": p})
}
func (s *Server) readVoiceAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	v := s.voiceService()
	if v == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	versions, err := v.ReadVoice(r.Context(), actor, tenant, employeeID)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"versions": versions})
}
func (s *Server) saveVoiceAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	var body struct {
		Version       int64              `json:"version"`
		Config        domain.VoiceConfig `json:"config"`
		ChangeSummary string             `json:"change_summary"`
	}
	if err = decodeEmployeeLarge(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	v := s.voiceService()
	if v == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	saved, err := v.SaveVoice(r.Context(), api.VoiceCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: body.Version, Config: body.Config, ChangeSummary: body.ChangeSummary})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(saved)
}
func (s *Server) previewVoiceAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	var body struct {
		VoiceVersion           int64  `json:"voice_version"`
		Text                   string `json:"text"`
		Slow                   bool   `json:"slow"`
		SyntheticDataConfirmed bool   `json:"synthetic_data_confirmed"`
	}
	if err = decodeEmployee(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	v := s.voiceService()
	if v == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	preview, err := v.PreviewVoice(r.Context(), api.VoicePreviewRequest{Actor: actor, TenantID: tenant, EmployeeID: employeeID, VoiceVersion: body.VoiceVersion, Text: body.Text, Slow: body.Slow, SyntheticDataConfirmed: body.SyntheticDataConfirmed})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrForbidden) {
			s.employeeProblem(w, r, err)
			return
		}
		var rule domain.VoiceRuleError
		if errors.As(err, &rule) && rule.Rule != "preview capability" {
			s.employeeProblem(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": err.Error(), "preview": preview})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(preview)
}
func (s *Server) saveVoiceForm(w http.ResponseWriter, r *http.Request) {
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
	tempo, err := strconv.Atoi(r.FormValue("tempo"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	v := s.voiceService()
	if v == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	profiles, err := v.VoiceCatalog(r.Context(), actor, tenant)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	profileID := r.FormValue("profile_id")
	language := ""
	for _, p := range profiles {
		if p.ID == profileID {
			language = p.Language
			break
		}
	}
	if language == "" {
		s.employeeProblem(w, r, domain.VoiceRuleError{Rule: "profile", Reason: "Неизвестный профиль голоса."})
		return
	}
	config := domain.VoiceConfig{ProfileID: profileID, Language: language, Style: r.FormValue("style"), Tempo: tempo}
	versions, err := v.ReadVoice(r.Context(), actor, tenant, employeeID)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	if len(versions) > 0 {
		for i, entry := range versions[0].Config.Dictionary {
			if r.FormValue("delete_entry_"+strconv.Itoa(i)) != "on" {
				config.Dictionary = append(config.Dictionary, entry)
			}
		}
	}
	if r.FormValue("written") != "" {
		entry := domain.Pronunciation{Written: r.FormValue("written"), Language: language, Scope: r.FormValue("scope"), Normalized: r.FormValue("normalized"), TestPhrase: r.FormValue("test_phrase"), SupportStatus: "unverified", CriticalKind: r.FormValue("critical_kind"), SyntheticDataConfirmed: r.FormValue("synthetic_data_confirmed") == "on"}
		if entry.CriticalKind != "none" && entry.CriticalKind != "" {
			entry.Readback = &domain.ReadbackPlan{Segments: splitExactSegments(r.FormValue("readback_segments")), Confirmation: "ask_customer", OnMismatch: "human_handoff", PhoneQualityGate: "pending_human"}
		}
		config.Dictionary = append(config.Dictionary, entry)
	}
	corr, _ := correlation.FromContext(r.Context())
	_, err = v.SaveVoice(r.Context(), api.VoiceCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, CorrelationID: corr, ExpectedVersion: version, Config: config, ChangeSummary: r.FormValue("change_summary")})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employeeID.String()+"?saved=1", http.StatusSeeOther)
}

func splitExactSegments(value string) []string {
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(value, "\r\n", "\n"), "\n"), "\n")
}

func (s *Server) readVoiceAssessmentsAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	v := s.voiceService()
	if v == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	assessments, err := v.ReadVoiceAssessments(r.Context(), actor, tenant, employeeID)
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"assessments": assessments})
}

func (s *Server) assessVoiceAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.employeeScope(w, r)
	if !ok {
		return
	}
	employeeID, err := id.Parse(r.PathValue("employee_id"))
	if err != nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	var body struct {
		PreviewID    string `json:"preview_id"`
		VoiceVersion int64  `json:"voice_version"`
		Verdict      string `json:"verdict"`
	}
	if err = decodeEmployee(w, r, &body); err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	previewID, err := id.Parse(body.PreviewID)
	if err != nil {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	v := s.voiceService()
	if v == nil {
		s.employeeProblem(w, r, domain.ErrNotFound)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	assessment, err := v.AssessVoice(r.Context(), api.VoiceAssessmentCommand{Actor: actor, TenantID: tenant, EmployeeID: employeeID, PreviewID: previewID, CorrelationID: corr, VoiceVersion: body.VoiceVersion, Verdict: body.Verdict})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessment)
}
