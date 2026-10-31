package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/tools"
)

// ---------------------------------------------------------------------------
// The Story 7.2 step-04 transport: the read-only catalog surface of the
// typed tool registry. Two machine GET paths — the list of the newest
// version of every contract and the pin read of one exact version (the
// material for future Manifest/Planner references — 8.3/7.3) — and the
// «Реестр инструментов» page. The read gate runs FIRST in the service: a
// non-member of the tenant is the honest 404, a member without the
// integrations|configuration scopes is the 403 with the scope words
// BEFORE any registry material (the Story 3.1 precedent). The path
// parameters are decoded strictly: a malformed tool id or version is the
// honest 404 of an invented pin — the pinned past stays resolvable, the
// malformed future does not exist. The page renders the honest empty
// state with the reason words when the registry is empty (AG-4: the
// empty registry is a state, not a defect), and the schemas of every
// card live behind the progressive disclosure of details/summary.
// ---------------------------------------------------------------------------

// toolsCatalogScope extracts the principal and the tenant path value of
// the catalog reads. An unauthenticated request is the 401 of the
// resolver; an unparseable tenant id is the honest 404.
func (s *Server) toolsCatalogScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.toolsCatalogProblem(w, r, tools.ErrNotFound)
		return actor, "", false
	}
	return actor, tenant, true
}

// toolsCatalogProblem maps the tool catalog errors onto the problem
// contract: the module's ErrNotFound is the plain 404 (a non-member or an
// unknown pin — no disclosure that the tenant has a tool surface); the
// module's ErrForbidden is the 403 whose message IS the scope words the
// service attached at the wrap point (the words of the two scopes that DO
// open the surface, spoken BEFORE any material — the honest words travel
// with the error, the transport never invents its own copy); anything
// else is the honest 500 with the server-side log.
func (s *Server) toolsCatalogProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, tools.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Ресурс не найден.", false, nil)
	case errors.Is(err, tools.ErrForbidden):
		// The service wrap is fmt.Errorf("%w: %s", ErrForbidden, scopeWords):
		// strip the sentinel prefix and speak the scope words themselves.
		message := err.Error()
		if prefix := tools.ErrForbidden.Error() + ": "; strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
		}
		s.problem(w, r, http.StatusForbidden, "tools_forbidden", message, false, nil)
	default:
		s.logger.ErrorContext(r.Context(), "tool catalog read failed", "code", "tool_catalog_failed", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось прочитать реестр инструментов. Повторите позднее.", true, nil)
	}
}

// toolsCatalogService is the narrow transport contract of the tool
// catalog: the list of the newest versions and the pin read of one exact
// version (the app tools.Service carries the compile-time proof; the
// transport tests inject a fake — the Story 3.1 precedent).
type toolsCatalogService interface {
	List(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID) ([]tools.ToolContract, error)
	Get(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, toolID, version string) (tools.ToolContract, error)
}

// BindToolsCatalog registers the read-only tool catalog surface: the
// machine list, the machine pin read and the registry page.
func (s *Server) BindToolsCatalog(service toolsCatalogService) {
	s.toolsCatalog = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/tools", s.toolsCatalogListAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/tools/{tool_id}/{tool_version}", s.toolsCatalogGetAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/tools", s.toolsPage)
}

// toolsCatalogListAPI answers the machine list: the newest version of
// every contract of the embedded registry. The honest empty list is the
// 200 with the empty array (never null) — the empty registry is a state,
// not a defect (AG-4).
func (s *Server) toolsCatalogListAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.toolsCatalogScope(w, r)
	if !ok {
		return
	}
	if s.toolsCatalog == nil {
		s.toolsCatalogProblem(w, r, tools.ErrNotFound)
		return
	}
	contracts, err := s.toolsCatalog.List(r.Context(), actor, tenant)
	if err != nil {
		s.toolsCatalogProblem(w, r, err)
		return
	}
	views := make([]toolContractView, 0, len(contracts))
	for _, contract := range contracts {
		views = append(views, newToolContractView(contract))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"tools": views})
}

// toolsCatalogGetAPI answers the machine pin read: one exact (id,
// version) pair. The path parameters are decoded strictly — a malformed
// identifier or a non-triple version is the honest 404 of a pin that
// never existed (strict decode before the service; an unknown
// well-formed pin is the service's own 404).
func (s *Server) toolsCatalogGetAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.toolsCatalogScope(w, r)
	if !ok {
		return
	}
	toolID, version, decodeOK := decodeToolContractPin(r)
	if !decodeOK {
		s.toolsCatalogProblem(w, r, tools.ErrNotFound)
		return
	}
	if s.toolsCatalog == nil {
		s.toolsCatalogProblem(w, r, tools.ErrNotFound)
		return
	}
	contract, err := s.toolsCatalog.Get(r.Context(), actor, tenant, toolID, version)
	if err != nil {
		s.toolsCatalogProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"tool": newToolContractView(contract)})
}

// decodeToolContractPin strictly decodes the two path parameters of the
// pin read. The identifier is 1..120 runes of the id charset; the version
// is the numeric dotted triple X.Y.Z (the registry publishes semver-core
// only — the ordering of versions stays total).
func decodeToolContractPin(r *http.Request) (string, string, bool) {
	toolID := r.PathValue("tool_id")
	version := r.PathValue("tool_version")
	if len([]rune(toolID)) < 1 || len([]rune(toolID)) > 120 {
		return "", "", false
	}
	for _, r := range toolID {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return "", "", false
		}
	}
	if !parseableToolVersion(version) {
		return "", "", false
	}
	return toolID, version, true
}

// parseableToolVersion checks the numeric dotted triple X.Y.Z without
// leading zeros — the strict semver-core word the registry publishes.
func parseableToolVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		if len(part) > 1 && part[0] == '0' {
			return false
		}
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// The machine DTO — ToolContractView (the OpenAPI contract schema)
// ---------------------------------------------------------------------------

// toolSchemaFieldView is one typed schema key of the contract.
type toolSchemaFieldView struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Values   []string `json:"values,omitempty"`
	Min      int      `json:"min,omitempty"`
	Max      int      `json:"max,omitempty"`
	Optional bool     `json:"optional"`
}

// toolDependencyView is the declared connector type dependency (nil for
// the self-contained contracts).
type toolDependencyView struct {
	ConnectorType string `json:"connector_type"`
}

// toolContractView is the read DTO of one tool contract — the transport
// maps the registry contract onto this view, never the other way around.
// The field names are the OpenAPI ToolContractView contract; the
// dictionary words (determinism_class, adapter_status) are pinned by the
// OpenAPI contract test against the platform dictionaries (drift turns
// the test red).
type toolContractView struct {
	ID               string                `json:"id"`
	Version          string                `json:"version"`
	Name             string                `json:"name"`
	Purpose          string                `json:"purpose"`
	DeterminismClass string                `json:"determinism_class"`
	AdapterStatus    string                `json:"adapter_status"`
	Owner            string                `json:"owner"`
	EvidenceRef      string                `json:"evidence_ref"`
	Dependency       *toolDependencyView   `json:"dependency,omitempty"`
	InputSchema      []toolSchemaFieldView `json:"input_schema"`
	OutputSchema     []toolSchemaFieldView `json:"output_schema"`
}

// newToolContractView maps one registry contract onto the contract DTO.
// The schemas are copied onto fresh slices in the dictionary order of the
// artifact (Go sorts map keys — the view order is the alphabetical key
// order, stable across reads); the empty schema renders as the empty
// array, never null.
func newToolContractView(contract tools.ToolContract) toolContractView {
	view := toolContractView{
		ID:               contract.ID,
		Version:          contract.Version,
		Name:             contract.Name,
		Purpose:          contract.Purpose,
		DeterminismClass: contract.DeterminismClass,
		AdapterStatus:    contract.AdapterStatus,
		Owner:            contract.Owner,
		EvidenceRef:      contract.EvidenceRef,
		InputSchema:      []toolSchemaFieldView{},
		OutputSchema:     []toolSchemaFieldView{},
	}
	if contract.Dependency != nil {
		view.Dependency = &toolDependencyView{ConnectorType: contract.Dependency.ConnectorType}
	}
	appendSchema := func(schema map[string]tools.ToolSchemaField, target *[]toolSchemaFieldView) {
		names := make([]string, 0, len(schema))
		for name := range schema {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			field := schema[name]
			entry := toolSchemaFieldView{
				Name: name, Label: field.Label, Type: field.Type,
				Min: field.Min, Max: field.Max, Optional: field.Optional,
			}
			if len(field.Values) > 0 {
				entry.Values = append([]string{}, field.Values...)
			}
			*target = append(*target, entry)
		}
	}
	appendSchema(contract.InputSchema, &view.InputSchema)
	appendSchema(contract.OutputSchema, &view.OutputSchema)
	return view
}

// ---------------------------------------------------------------------------
// The «Реестр инструментов» page
// ---------------------------------------------------------------------------

// toolCardView is the flattened card projection of one contract: the
// headline words render textually (name, version, class, adapter status,
// dependency, owner); the typed schemas live behind the progressive
// disclosure — the card never floods the reader with every key at once.
type toolCardView struct {
	ID             string
	Version        string
	Name           string
	Purpose        string
	Determinism    string
	AdapterStatus  string
	Owner          string
	EvidenceRef    string
	DependencyText string
	HasInputKeys   bool
	InputRows      []toolFieldRow
	HasOutputKeys  bool
	OutputRows     []toolFieldRow
}

// toolFieldRow is one typed key of the card disclosure.
type toolFieldRow struct {
	Name     string
	Label    string
	Type     string
	Optional string
	Bounds   string
	Values   string
}

// toolsPageView carries the page material: the tenant, the cards of the
// newest contract versions and the honest empty state (the reason words —
// never a bare stub page).
type toolsPageView struct {
	TenantID  string
	HasCards  bool
	Cards     []toolCardView
	EmptyText string
}

// toolsCatalogEmptyText is the honest reason of the empty registry
// (AG-4): the words explain WHY the registry is empty — the contracts
// arrive as append-only artifacts of the published packs — so the
// absence is a readable state, not a silent blank.
const toolsCatalogEmptyText = "Реестр инструментов пуст: контракты появляются как версионируемые артефакты, привязанные опубликованными Domain Pack'ами (путь внешнего автора — PR «артефакт + индексная запись»). Сегодня ни один пак не привязывает контракты — это честное состояние реестра, а не ошибка."

// toolsPage renders the «Реестр инструментов» page: the cards of the
// newest contract versions with the progressive disclosure of the typed
// schemas (details/summary — the provider-technique precedent), or the
// honest empty state with the reason words.
func (s *Server) toolsPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.toolsCatalogScope(w, r)
	if !ok {
		return
	}
	if s.toolsCatalog == nil {
		s.toolsCatalogProblem(w, r, tools.ErrNotFound)
		return
	}
	contracts, err := s.toolsCatalog.List(r.Context(), actor, tenant)
	if err != nil {
		s.toolsCatalogProblem(w, r, err)
		return
	}
	data := toolsPageView{TenantID: tenant.String()}
	if len(contracts) > 0 {
		data.HasCards = true
		for _, contract := range contracts {
			data.Cards = append(data.Cards, newToolCardView(contract))
		}
	} else {
		data.EmptyText = toolsCatalogEmptyText
	}
	s.render(w, "tools", data)
}

// newToolCardView flattens one contract onto the card projection. Every
// dictionary word renders textually; the honest not_implemented state is
// stated on the card (a declared contract is visible but callable by
// nobody — the double honesty of the slice).
func newToolCardView(contract tools.ToolContract) toolCardView {
	card := toolCardView{
		ID:            contract.ID,
		Version:       contract.Version,
		Name:          contract.Name,
		Purpose:       contract.Purpose,
		Determinism:   contract.DeterminismClass,
		AdapterStatus: contract.AdapterStatus,
		Owner:         contract.Owner,
		EvidenceRef:   contract.EvidenceRef,
	}
	if contract.Dependency != nil {
		card.DependencyText = contract.Dependency.ConnectorType
	}
	card.HasInputKeys = len(contract.InputSchema) > 0
	card.HasOutputKeys = len(contract.OutputSchema) > 0
	card.InputRows = toolFieldRows(contract.InputSchema)
	card.OutputRows = toolFieldRows(contract.OutputSchema)
	return card
}

// toolFieldRows maps the typed schema keys onto the card rows (the
// alphabetical key order — stable across reads).
func toolFieldRows(schema map[string]tools.ToolSchemaField) []toolFieldRow {
	names := make([]string, 0, len(schema))
	for name := range schema {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([]toolFieldRow, 0, len(names))
	for _, name := range names {
		field := schema[name]
		row := toolFieldRow{
			Name:     name,
			Label:    field.Label,
			Type:     field.Type,
			Optional: toolYesNo(field.Optional),
		}
		if field.Type == "integer" && (field.Min != 0 || field.Max != 0) {
			row.Bounds = strconv.Itoa(field.Min) + "–" + strconv.Itoa(field.Max)
		}
		if len(field.Values) > 0 {
			row.Values = strings.Join(field.Values, "; ")
		}
		rows = append(rows, row)
	}
	return rows
}

// toolYesNo renders the honest yes/no word of the boolean card words.
func toolYesNo(value bool) string {
	if value {
		return "да"
	}
	return "нет"
}
