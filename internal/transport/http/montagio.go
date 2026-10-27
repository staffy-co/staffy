package httptransport

import (
	"encoding/json"
	"net/http"

	employeeAPI "github.com/artemiosu/staffy/internal/employee/api"
	employeeDomain "github.com/artemiosu/staffy/internal/employee/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// bindMontagio mounts the Montagio Admission Registry surfaces of spec 5-1:
// the machine read-only view and the accessible pilot page. The rights are
// the reused test & release view scope (configuration manages, review/audit
// view); membership resolves as the honest 404 before any material.
func (s *Server) bindMontagio() {
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/montagio/admission", s.montagioAdmissionAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/montagio", s.montagioPage)
}

// montagioAdmissionScope resolves the principal and the tenant path value of
// the Montagio surface (no employee dimension: the admission is a
// tenant-level derivation over the sealed manifests, the connector facts
// and the gate instance history of the tenant's employees).
func (s *Server) montagioAdmissionScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return actor, "", false
	}
	return actor, tenant, true
}

// montagioAdmissionAPI answers the machine view: the read-time derivation of
// the Montagio admission (200) with the honest problem ladder (401/403/404).
// The read is empty-bodied — a GET without a body never yields a 400, and
// unknown query fields stay ignored (the strict-decode discipline applies
// to command bodies, never to this view).
func (s *Server) montagioAdmissionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.montagioAdmissionScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.MontagioAdmission(r.Context(), actor, tenant)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

// montagioPage renders the accessible Montagio pilot page: the Route Matrix
// with the four stages as text (never color or position alone), every
// blocker with the exact word, the safe state and the next owner, the
// boundary fence words and the approved AC-11 wording dictionary as a
// section. Secrets and credentials never travel into the page — the view
// reads the derived facts only.
func (s *Server) montagioPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.montagioAdmissionScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.MontagioAdmission(r.Context(), actor, tenant)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	s.render(w, "montagio", map[string]any{
		"TenantID": tenant,
		"View":     prepareMontagioAdmissionView(view),
	})
}

// ---------------------------------------------------------------------------
// The page projection
// ---------------------------------------------------------------------------

// montagioPageView is the flattened projection the template renders.
type montagioPageView struct {
	CanManage    bool
	ScopeText    string
	PackText     string
	PackOK       bool
	WordingWords []string
	Routes       []montagioRouteRow
}

// montagioRouteRow is one route of the Route Matrix: the stage as text, the
// honest stage word, the column words, the requirement views and the
// blockers with their exact words.
type montagioRouteRow struct {
	ID                string
	Title             string
	Stage             string
	StageText         string
	StageWord         string
	ModalityWord      string
	IntentsWord       string
	AutonomyWord      string
	HumanInboxWord    string
	FallbackRouteWord string
	CapWord           string
	Capabilities      []montagioCapabilityRow
	Gates             []montagioGateRow
	LiveGates         []montagioGateRow
	Blockers          []montagioBlockerRow
	FenceWords        []string
	FallbackWords     []string
}

// montagioCapabilityRow is one required capability with its derived state.
type montagioCapabilityRow struct {
	ID          string
	ConnectorID string
	State       string
	Word        string
}

// montagioGateRow is one required gate with its derived state and words.
type montagioGateRow struct {
	ID        string
	State     string
	StateText string
	Word      string
	SafeState string
	NextOwner string
}

// montagioBlockerRow is one honest blocking of a route.
type montagioBlockerRow struct {
	Source    string
	ID        string
	Word      string
	SafeState string
	NextOwner string
}

// prepareMontagioAdmissionView converts the service view into the page
// projection (the stages read as text; every blocker carries its exact
// word, safe state and next owner).
func prepareMontagioAdmissionView(view employeeAPI.MontagioAdmissionView) montagioPageView {
	data := montagioPageView{
		CanManage:    view.CanManage,
		ScopeText:    readinessScopeText(view.ViewScope),
		PackText:     view.Report.PackWord,
		PackOK:       view.Report.PackPinned,
		WordingWords: view.Report.WordingWords,
		Routes:       []montagioRouteRow{},
	}
	for _, route := range view.Report.Routes {
		row := montagioRouteRow{
			ID: route.RouteID, Title: route.Title,
			Stage: route.Stage, StageText: montagioStageText(route.Stage), StageWord: route.StageWord,
			ModalityWord: route.ModalityWord, IntentsWord: route.IntentsWord,
			AutonomyWord: route.AutonomyWord, HumanInboxWord: route.HumanInboxWord,
			FallbackRouteWord: route.FallbackRouteWord, CapWord: route.CapWord,
			Capabilities: []montagioCapabilityRow{}, Gates: []montagioGateRow{}, LiveGates: []montagioGateRow{}, Blockers: []montagioBlockerRow{},
			FenceWords: route.FenceWords, FallbackWords: route.FallbackWords,
		}
		for _, capability := range route.Capabilities {
			row.Capabilities = append(row.Capabilities, montagioCapabilityRow{
				ID: capability.ID, ConnectorID: capability.ConnectorID,
				State: capability.State, Word: capability.Word,
			})
		}
		for _, gate := range route.Gates {
			row.Gates = append(row.Gates, montagioGateRow{
				ID: gate.ID, State: gate.State, StateText: pilotGateStateText(gate.State),
				Word: gate.Word, SafeState: gate.SafeState, NextOwner: gate.NextOwner,
			})
		}
		for _, gate := range route.LiveGates {
			row.LiveGates = append(row.LiveGates, montagioGateRow{
				ID: gate.ID, State: gate.State, StateText: pilotGateStateText(gate.State),
				Word: gate.Word, SafeState: gate.SafeState, NextOwner: gate.NextOwner,
			})
		}
		for _, blocker := range route.Blockers {
			row.Blockers = append(row.Blockers, montagioBlockerRow{
				Source: blocker.Source, ID: blocker.ID, Word: blocker.Word,
				SafeState: blocker.SafeState, NextOwner: blocker.NextOwner,
			})
		}
		data.Routes = append(data.Routes, row)
	}
	return data
}

// montagioStageText renders the admission stage honestly for displays (the
// stage reads as text — AC-13: never color or position alone; the four
// stages are distinguishable in words and semantically).
func montagioStageText(stage string) string {
	switch stage {
	case employeeDomain.MontagioStageCandidate:
		return "кандидат"
	case employeeDomain.MontagioStageVerified:
		return "проверен"
	case employeeDomain.MontagioStageControlled:
		return "контролируемый реальный клиент"
	case employeeDomain.MontagioStageLive:
		return "live"
	}
	return stage
}
