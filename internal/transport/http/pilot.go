package httptransport

import (
	"encoding/json"
	"net/http"

	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// bindPilot mounts the Pilot Admission Registry surfaces of spec 4-1: the
// machine read-only view and the accessible pilot page. The rights are the
// reused test & release view scope (configuration manages, review/audit
// view); membership resolves as the honest 404 before any material.
func (s *Server) bindPilot() {
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/pilot/admission", s.pilotAdmissionAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/pilot", s.pilotPage)
}

// pilotAdmissionScope resolves the principal and the tenant path value of
// the pilot surface (no employee dimension: the admission is a tenant-level
// derivation over the sealed manifests, the connector facts and the gate
// instance history of the tenant's employees).
func (s *Server) pilotAdmissionScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
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

// pilotAdmissionAPI answers the machine view: the read-time derivation of
// the pilot admission (200) with the honest problem ladder (401/403/404).
// The read is empty-bodied — a GET without a body never yields a 400, and
// unknown query fields stay ignored (the strict-decode discipline applies
// to command bodies, never to this view).
func (s *Server) pilotAdmissionAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.pilotAdmissionScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.PilotAdmission(r.Context(), actor, tenant)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

// pilotPage renders the accessible pilot page: the route matrix with the
// stages as text (never color or position alone), every blocker with the
// exact word, the safe state and the next owner, the order fence words and
// the ABCP→1C matrix in closed details. Secrets and credentials never
// travel into the page — the view reads the derived facts only.
func (s *Server) pilotPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.pilotAdmissionScope(w, r)
	if !ok {
		return
	}
	service := s.readinessService()
	if service == nil {
		s.readinessProblem(w, r, employeeDomain.ErrNotFound)
		return
	}
	view, err := service.PilotAdmission(r.Context(), actor, tenant)
	if err != nil {
		s.readinessProblem(w, r, err)
		return
	}
	s.render(w, "pilot", map[string]any{
		"TenantID": tenant,
		"View":     preparePilotAdmissionView(view),
	})
}

// ---------------------------------------------------------------------------
// The page projection
// ---------------------------------------------------------------------------

// pilotPageView is the flattened projection the template renders.
type pilotPageView struct {
	CanManage bool
	ScopeText string
	PackText  string
	PackOK    bool
	Routes    []pilotRouteRow
	Matrix    []pilotMatrixRow
}

// pilotRouteRow is one route of the matrix: the stage as text, the honest
// stage word, the requirement views and the blockers with their exact
// words.
type pilotRouteRow struct {
	ID            string
	Title         string
	Stage         string
	StageText     string
	StageWord     string
	Capabilities  []pilotCapabilityRow
	Gates         []pilotGateRow
	Blockers      []pilotBlockerRow
	FenceWords    []string
	FallbackWords []string
}

// pilotCapabilityRow is one required capability with its derived state.
type pilotCapabilityRow struct {
	ID          string
	ConnectorID string
	State       string
	Word        string
}

// pilotGateRow is one required gate with its derived state and words.
type pilotGateRow struct {
	ID        string
	State     string
	StateText string
	Word      string
	SafeState string
	NextOwner string
}

// pilotBlockerRow is one honest blocking of a route.
type pilotBlockerRow struct {
	Source    string
	ID        string
	Word      string
	SafeState string
	NextOwner string
}

// pilotMatrixRow is one material field of the ABCP→1C matrix.
type pilotMatrixRow struct {
	Field        string
	Title        string
	OwningSystem string
	ExternalID   string
	VersionTime  string
	Projection   string
	ConflictRule string
	Reconcile    string
	Terminal     string
	StateWord    string
}

// preparePilotAdmissionView converts the service view into the page
// projection (the stages read as text; every blocker carries its exact
// word, safe state and next owner).
func preparePilotAdmissionView(view employeeAPI.PilotAdmissionView) pilotPageView {
	data := pilotPageView{
		CanManage: view.CanManage,
		ScopeText: readinessScopeText(view.ViewScope),
		PackText:  view.Report.PackWord,
		PackOK:    view.Report.PackPinned,
		Routes:    []pilotRouteRow{},
		Matrix:    []pilotMatrixRow{},
	}
	for _, route := range view.Report.Routes {
		row := pilotRouteRow{
			ID: route.RouteID, Title: route.Title,
			Stage: route.Stage, StageText: pilotStageText(route.Stage), StageWord: route.StageWord,
			Capabilities: []pilotCapabilityRow{}, Gates: []pilotGateRow{}, Blockers: []pilotBlockerRow{},
			FenceWords: route.FenceWords, FallbackWords: route.FallbackWords,
		}
		for _, capability := range route.Capabilities {
			row.Capabilities = append(row.Capabilities, pilotCapabilityRow{
				ID: capability.ID, ConnectorID: capability.ConnectorID,
				State: capability.State, Word: capability.Word,
			})
		}
		for _, gate := range route.Gates {
			row.Gates = append(row.Gates, pilotGateRow{
				ID: gate.ID, State: gate.State, StateText: pilotGateStateText(gate.State),
				Word: gate.Word, SafeState: gate.SafeState, NextOwner: gate.NextOwner,
			})
		}
		for _, blocker := range route.Blockers {
			row.Blockers = append(row.Blockers, pilotBlockerRow{
				Source: blocker.Source, ID: blocker.ID, Word: blocker.Word,
				SafeState: blocker.SafeState, NextOwner: blocker.NextOwner,
			})
		}
		data.Routes = append(data.Routes, row)
	}
	for _, field := range view.Report.Matrix {
		data.Matrix = append(data.Matrix, pilotMatrixRow{
			Field: field.Field, Title: field.Title, OwningSystem: field.OwningSystem,
			ExternalID: field.ExternalIDText, VersionTime: field.VersionTimeText,
			Projection: field.AllowedProjectionText, ConflictRule: field.ConflictRuleText,
			Reconcile: field.ReconcileQueryText, Terminal: field.TerminalEvidenceText,
			StateWord: field.StateWord,
		})
	}
	return data
}

// pilotStageText renders the admission stage honestly for displays (the
// stage reads as text — AC-13: never color or position alone).
func pilotStageText(stage string) string {
	switch stage {
	case employeeDomain.PilotStageCandidate:
		return "кандидат"
	case employeeDomain.PilotStageVerified:
		return "проверен"
	case employeeDomain.PilotStageAdmitted:
		return "допущен"
	}
	return stage
}

// pilotGateStateText renders one gate requirement state.
func pilotGateStateText(state string) string {
	if text := employeeAPI.GateInstanceStateText(state); text != state {
		return text
	}
	switch state {
	case "missing":
		return "записи нет"
	case "unproven":
		return "не доказано"
	}
	return state
}
