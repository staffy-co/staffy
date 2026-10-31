package httptransport

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/staffy-co/staffy/internal/employee/api"
	"github.com/staffy-co/staffy/internal/employee/domain"
	"github.com/staffy-co/staffy/internal/employee/ingest"
	"github.com/staffy-co/staffy/internal/platform/correlation"
)

func (s *Server) ingestKnowledgeAPI(w http.ResponseWriter, r *http.Request) {
	s.ingestKnowledge(w, r, false)
}
func (s *Server) ingestKnowledgeForm(w http.ResponseWriter, r *http.Request) {
	s.ingestKnowledge(w, r, true)
}

func (s *Server) reingestFixtureAPI(w http.ResponseWriter, r *http.Request) {
	s.reingestFixture(w, r, false)
}
func (s *Server) reingestFixtureForm(w http.ResponseWriter, r *http.Request) {
	s.reingestFixture(w, r, true)
}
func (s *Server) reingestFixture(w http.ResponseWriter, r *http.Request, form bool) {
	tenant, employee, sourceID, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	if form && !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	var body struct {
		Version   int64  `json:"version"`
		FixtureID string `json:"fixture_id"`
	}
	if form {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
		version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
		if err != nil {
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
		body.Version, body.FixtureID = version, r.FormValue("fixture_id")
	} else {
		if err := decodeEmployee(w, r, &body); err != nil {
			s.employeeProblem(w, r, err)
			return
		}
	}
	corr, _ := correlation.FromContext(r.Context())
	source, version, replayed, err := s.ingestion.ReingestSyntheticKnowledge(r.Context(), api.KnowledgeFixtureCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, SourceID: sourceID, CorrelationID: corr, ExpectedVersion: body.Version, FixtureID: body.FixtureID})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	if form {
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employee.String(), http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"source": source, "version": version, "replayed": replayed, "publishable": false})
}

func (s *Server) ingestKnowledge(w http.ResponseWriter, r *http.Request, form bool) {
	tenant, employee, _, ok := s.knowledgeIDs(w, r)
	if !ok {
		return
	}
	if form && !validFormOrigin(r) {
		s.employeeProblem(w, r, domain.ErrForbidden)
		return
	}
	actor, ok := s.principal(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, ingest.MaxFileBytes+(16<<10))
	var body struct {
		Type       string `json:"type"`
		Title      string `json:"title"`
		Owner      string `json:"owner"`
		Provenance string `json:"provenance"`
		Scope      string `json:"scope"`
		Priority   int    `json:"priority"`
		Language   string `json:"language"`
		URL        string `json:"url"`
		Fixture    bool   `json:"fixture"`
	}
	var filename string
	var file []byte
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if !validFormOrigin(r) {
			s.employeeProblem(w, r, domain.ErrForbidden)
			return
		}
		if err := r.ParseMultipartForm(ingest.MaxFileBytes + (16 << 10)); err != nil {
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
		defer r.MultipartForm.RemoveAll()
		if r.URL.RawQuery != "" {
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
		for _, values := range r.MultipartForm.Value {
			if len(values) != 1 {
				s.employeeProblem(w, r, domain.ErrInvalid)
				return
			}
		}
		body.Type, body.Title, body.Owner = r.FormValue("type"), r.FormValue("title"), r.FormValue("owner")
		body.Provenance, body.Scope, body.Language = r.FormValue("provenance"), r.FormValue("scope"), r.FormValue("language")
		body.URL, body.Fixture = r.FormValue("url"), r.FormValue("fixture") == "true"
		priority, err := strconv.Atoi(r.FormValue("priority"))
		if err != nil {
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
		body.Priority = priority
		files := r.MultipartForm.File["file"]
		if len(files) > 1 {
			s.employeeProblem(w, r, domain.ErrInvalid)
			return
		}
		if len(files) == 1 {
			filename = files[0].Filename
			stream, err := files[0].Open()
			if err != nil {
				s.employeeProblem(w, r, domain.ErrInvalid)
				return
			}
			file, err = io.ReadAll(io.LimitReader(stream, ingest.MaxFileBytes+1))
			stream.Close()
			if err != nil {
				s.employeeProblem(w, r, domain.ErrInvalid)
				return
			}
		}
	} else if !form {
		if err := decodeEmployee(w, r, &body); err != nil {
			s.employeeProblem(w, r, err)
			return
		}
	} else {
		s.employeeProblem(w, r, domain.ErrInvalid)
		return
	}
	corr, _ := correlation.FromContext(r.Context())
	source, version, err := s.ingestion.IngestKnowledge(r.Context(), api.KnowledgeIngestCommand{Actor: actor, TenantID: tenant, EmployeeID: employee, CorrelationID: corr, Source: domain.KnowledgeSource{Type: body.Type, Title: body.Title, Owner: body.Owner, Provenance: body.Provenance, Scope: body.Scope, Priority: body.Priority, Language: body.Language}, URL: body.URL, FileName: filename, File: file, Fixture: body.Fixture})
	if err != nil {
		s.employeeProblem(w, r, err)
		return
	}
	if form {
		http.Redirect(w, r, "/app/tenants/"+tenant.String()+"/employees/"+employee.String(), http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"source": source, "version": version, "publishable": false})
}
