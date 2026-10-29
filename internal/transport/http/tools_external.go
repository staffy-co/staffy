package httptransport

// ---------------------------------------------------------------------------
// The Story 7.4 step-03 transport: the read-only external tools dictionary
// surface. Two machine GET paths — the API list of the derived sections
// and the operator page — behind the membership ladder of the platform:
// no identity is the 401 of the resolver, an unparseable tenant id and a
// non-member are the honest 404, a member without the
// integrations|configuration scopes is the 403 with the scope words BEFORE
// any dictionary material (the 3.1/7.2 precedent). The surface is STRICTLY
// read-only: no call forms, no probe buttons — the probes live on the
// existing connection page of 1-8, the caller of the tools is the future
// Planner 8.3. The honest empty state carries the reason words (no
// connection of the class exists — a state, not a defect).
// ---------------------------------------------------------------------------

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/artemiosu/staffy/internal/externaltools"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/tools"
)

// externalToolsService is the narrow transport contract of the dictionary
// (the app externaltools.Service carries the compile-time proof; the
// transport tests inject a fake — the Story 3.1 precedent).
type externalToolsService interface {
	Dictionary(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]externaltools.Section, error)
}

// BindExternalTools registers the read-only external tools dictionary
// surface: the machine list and the operator page.
func (s *Server) BindExternalTools(service externalToolsService) {
	s.externalTools = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/tools/external", s.externalToolsListAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/tools/external", s.externalToolsPage)
}

// externalToolsScope extracts the principal and the tenant path value of
// the dictionary reads. An unauthenticated request is the 401 of the
// resolver; an unparseable tenant id is the honest 404.
func (s *Server) externalToolsScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.externalToolsProblem(w, r, externaltools.ErrNotFound)
		return actor, "", false
	}
	return actor, tenant, true
}

// externalToolsProblem maps the dictionary errors onto the problem
// contract: the module's ErrNotFound is the plain 404 (no disclosure that
// the tenant has an external tool dictionary); the module's ErrForbidden
// is the 403 whose message IS the scope words the service attached at the
// wrap point; anything else is the honest 500 with the server-side log.
func (s *Server) externalToolsProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, externaltools.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Ресурс не найден.", false, nil)
	case errors.Is(err, externaltools.ErrForbidden):
		message := err.Error()
		if prefix := externaltools.ErrForbidden.Error() + ": "; strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
		}
		s.problem(w, r, http.StatusForbidden, "external_tools_forbidden", message, false, nil)
	default:
		s.logger.ErrorContext(r.Context(), "external tools dictionary read failed", "code", "external_tools_failed", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось прочитать словарь внешних инструментов. Повторите позднее.", true, nil)
	}
}

// externalToolsListAPI answers the machine list: the derived sections of
// the live external MCP connections. The honest empty array (never null)
// is the state of a tenant without connections of the class.
func (s *Server) externalToolsListAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.externalToolsScope(w, r)
	if !ok {
		return
	}
	if s.externalTools == nil {
		s.externalToolsProblem(w, r, externaltools.ErrNotFound)
		return
	}
	sections, err := s.externalTools.Dictionary(r.Context(), actor, tenant)
	if err != nil {
		s.externalToolsProblem(w, r, err)
		return
	}
	views := make([]externalToolSectionView, 0, len(sections))
	for _, section := range sections {
		views = append(views, newExternalToolSectionView(section))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sections": views})
}

// ---------------------------------------------------------------------------
// The machine DTO — ExternalToolSectionView (the OpenAPI contract schema)
// ---------------------------------------------------------------------------

// externalToolProvenanceView is the snapshot identity of one entry: the
// class card pin, the external server identity, the evidence row and its
// validity window (the external tools carry no versions in the pinned
// revision — the provenance IS the version statement).
type externalToolProvenanceView struct {
	ConnectorID      string `json:"connector_id"`
	ConnectorVersion string `json:"connector_version"`
	ServerName       string `json:"server_name"`
	ServerVersion    string `json:"server_version"`
	EvidenceID       string `json:"evidence_id"`
	CheckedAt        string `json:"checked_at"`
	ExpiresAt        string `json:"expires_at"`
}

// externalToolEntryView is one row of the dictionary: the filtered
// descriptor with the projected typed schemas and the admission state.
type externalToolEntryView struct {
	Name         string                      `json:"name"`
	Title        string                      `json:"title"`
	Description  string                      `json:"description"`
	State        string                      `json:"state"`
	Reason       string                      `json:"reason,omitempty"`
	Provenance   *externalToolProvenanceView `json:"provenance,omitempty"`
	InputSchema  []toolSchemaFieldView       `json:"input_schema"`
	OutputSchema []toolSchemaFieldView       `json:"output_schema"`
}

// externalToolSectionView is the per-connection block of the dictionary.
type externalToolSectionView struct {
	ConnectionID     string                  `json:"connection_id"`
	ConnectorID      string                  `json:"connector_id"`
	ConnectorVersion string                  `json:"connector_version"`
	Environment      string                  `json:"environment"`
	Status           string                  `json:"status"`
	HasEntries       bool                    `json:"has_entries"`
	Reason           string                  `json:"reason,omitempty"`
	Entries          []externalToolEntryView `json:"entries"`
}

// newExternalToolSectionView maps one derived section onto the contract
// DTO: the schemas of the admitted rows ride the platform typing view (the
// ToolSchemaFieldView contract — the mirror of the registry discipline),
// the rejected rows carry the named refusal and no material.
func newExternalToolSectionView(section externaltools.Section) externalToolSectionView {
	view := externalToolSectionView{
		ConnectionID:     section.ConnectionID.String(),
		ConnectorID:      section.ConnectorID,
		ConnectorVersion: section.ContractVersion,
		Environment:      section.Environment,
		Status:           section.Status,
		HasEntries:       section.HasEntries,
		Reason:           section.Reason,
		Entries:          make([]externalToolEntryView, 0, len(section.Entries)),
	}
	for _, entry := range section.Entries {
		entryView := externalToolEntryView{
			Name:         entry.Name,
			Title:        entry.Title,
			Description:  entry.Description,
			State:        entry.State,
			Reason:       entry.Reason,
			InputSchema:  []toolSchemaFieldView{},
			OutputSchema: []toolSchemaFieldView{},
		}
		if entry.Provenance != nil {
			entryView.Provenance = &externalToolProvenanceView{
				ConnectorID:      entry.Provenance.ConnectorID,
				ConnectorVersion: entry.Provenance.ConnectorVersion,
				ServerName:       entry.Provenance.ServerName,
				ServerVersion:    entry.Provenance.ServerVersion,
				EvidenceID:       entry.Provenance.EvidenceID.String(),
				CheckedAt:        entry.Provenance.CheckedAt.Format("2006-01-02T15:04:05Z07:00"),
				ExpiresAt:        entry.Provenance.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			}
		}
		entryView.InputSchema = appendSchemaViews(entry.Input)
		entryView.OutputSchema = appendSchemaViews(entry.Output)
		view.Entries = append(view.Entries, entryView)
	}
	return view
}

// appendSchemaViews maps the projected typing dictionary onto the contract
// field views (the alphabetical key order — stable across reads; the row
// form of the tools page is the shared projection helper).
func appendSchemaViews(schema map[string]tools.ToolSchemaField) []toolSchemaFieldView {
	out := make([]toolSchemaFieldView, 0, len(schema))
	for _, row := range toolFieldRows(schema) {
		entry := toolSchemaFieldView{
			Name:     row.Name,
			Label:    row.Label,
			Type:     row.Type,
			Optional: row.Optional == "да",
		}
		if row.Bounds != "" {
			bounds := strings.SplitN(row.Bounds, "–", 2)
			if len(bounds) == 2 {
				entry.Min, _ = strconv.Atoi(bounds[0])
				entry.Max, _ = strconv.Atoi(bounds[1])
			}
		}
		if row.Values != "" {
			entry.Values = strings.Split(row.Values, "; ")
		}
		out = append(out, entry)
	}
	return out
}

// ---------------------------------------------------------------------------
// The «Словарь внешних инструментов» page
// ---------------------------------------------------------------------------

// externalToolCardView is the flattened card projection of one entry.
type externalToolCardView struct {
	Name          string
	Title         string
	Description   string
	Rejected      bool
	StateText     string
	Reason        string
	HasProvenance bool
	ServerText    string
	ConnectorPin  string
	EvidenceID    string
	CheckedAtText string
	ExpiresAtText string
	HasInputKeys  bool
	InputRows     []toolFieldRow
	HasOutputKeys bool
	OutputRows    []toolFieldRow
}

// externalSectionPageView is the per-connection block of the page.
type externalSectionPageView struct {
	ConnectionID string
	ConnectorPin string
	Environment  string
	StatusText   string
	HasEntries   bool
	Reason       string
	Cards        []externalToolCardView
}

// externalToolsPageView carries the page material: the tenant, the sections
// and the honest empty state (the reason words — never a bare stub).
type externalToolsPageView struct {
	TenantID    string
	HasSections bool
	Sections    []externalSectionPageView
	EmptyText   string
}

// externalToolsEmptyText is the honest reason of the empty dictionary: the
// words explain WHY no entries exist — no connection of the class has been
// created (or none holds applicable evidence) — so the absence is a
// readable state, not a silent blank.
const externalToolsEmptyText = "Словарь внешних инструментов пуст: записи появляются из применимых доказательств probe «Словарь инструментов сервера» живых подключений класса «внешний MCP-сервер» (страница «Интеграции»). Обновление — только живой командой оператора (повторная проба); фоновых обновлений нет."

// externalToolsPage renders the operator dictionary: the per-connection
// sections with the provenance of every entry and the honest states (a
// rejected row carries its named reason; an empty section carries the
// anchor words of the connector machine).
func (s *Server) externalToolsPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.externalToolsScope(w, r)
	if !ok {
		return
	}
	if s.externalTools == nil {
		s.externalToolsProblem(w, r, externaltools.ErrNotFound)
		return
	}
	sections, err := s.externalTools.Dictionary(r.Context(), actor, tenant)
	if err != nil {
		s.externalToolsProblem(w, r, err)
		return
	}
	data := externalToolsPageView{TenantID: tenant.String()}
	if len(sections) > 0 {
		data.HasSections = true
		for _, section := range sections {
			pageSection := externalSectionPageView{
				ConnectionID: section.ConnectionID.String(),
				ConnectorPin: section.ConnectorID + "@" + section.ContractVersion,
				Environment:  section.Environment,
				StatusText:   connectionStatusText(section.Status, "", "", section.Environment),
				HasEntries:   section.HasEntries,
				Reason:       section.Reason,
				Cards:        []externalToolCardView{},
			}
			for _, entry := range section.Entries {
				card := externalToolCardView{
					Name:        entry.Name,
					Title:       entry.Title,
					Description: entry.Description,
					Rejected:    !entry.Admitted(),
					StateText:   externalEntryStateText(entry.State),
					Reason:      entry.Reason,
				}
				if entry.Provenance != nil {
					card.HasProvenance = true
					card.ServerText = entry.Provenance.ServerName + " " + entry.Provenance.ServerVersion
					card.ConnectorPin = entry.Provenance.ConnectorID + "@" + entry.Provenance.ConnectorVersion
					card.EvidenceID = entry.Provenance.EvidenceID.String()
					card.CheckedAtText = entry.Provenance.CheckedAt.Format("02.01.2006 15:04 UTC")
					card.ExpiresAtText = entry.Provenance.ExpiresAt.Format("02.01.2006 15:04 UTC")
				}
				card.HasInputKeys = len(entry.Input) > 0
				card.HasOutputKeys = len(entry.Output) > 0
				card.InputRows = toolFieldRows(entry.Input)
				card.OutputRows = toolFieldRows(entry.Output)
				pageSection.Cards = append(pageSection.Cards, card)
			}
			data.Sections = append(data.Sections, pageSection)
		}
	} else {
		data.EmptyText = externalToolsEmptyText
	}
	s.render(w, "external_tools", data)
}

// externalEntryStateText renders the admission state as readable Russian.
func externalEntryStateText(state string) string {
	if state == externaltools.EntryAdmitted {
		return "допущен (read-only заявлен сервером)"
	}
	return "отклонён"
}
