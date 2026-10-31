package httptransport

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 3.1 step-03 transport: the read-only catalog surface of the
// authorized action dictionary. Two machine GET paths — the list of the
// newest version of every definition and the pin read of one exact
// version (AC-8: the material for future Manifest/Operation/Receipt
// references) — and the «Каталог действий» page. The read gate runs FIRST
// in the service: a non-member of the tenant is the honest 404, a member
// without the integrations|configuration scopes is the 403 with the scope
// words BEFORE any catalog material (the decision-context 2-14
// precedent). The path parameters are decoded strictly: a malformed
// definition id or version is the honest 404 of an invented pin — the
// pinned past stays resolvable, the malformed future does not exist. The
// page renders the honest empty state with the reason words when the
// dictionary is empty (AG-4: the empty catalog is a state, not a defect),
// and the conditions of every card live behind the progressive
// disclosure of details/summary (the provider-technique 2-14 precedent).
// ---------------------------------------------------------------------------

// actionCatalogScope extracts the principal and the tenant path value of
// the catalog reads. An unauthenticated request is the 401 of the
// resolver; an unparseable tenant id is the honest 404 (the connector
// scope precedent).
func (s *Server) actionCatalogScope(w http.ResponseWriter, r *http.Request) (identityDomain.Principal, id.UUID, bool) {
	actor, ok := s.principal(w, r)
	if !ok {
		return actor, "", false
	}
	tenant, err := id.Parse(r.PathValue("tenant_id"))
	if err != nil {
		s.actionCatalogProblem(w, r, actionDomain.ErrNotFound)
		return actor, "", false
	}
	return actor, tenant, true
}

// actionCatalogProblem maps the action catalog errors onto the problem
// contract: the module's ErrNotFound is the plain 404 (a non-member or
// an unknown pin — no disclosure that the tenant has an action catalog);
// the module's ErrForbidden is the 403 whose message IS the scope words
// the app layer attached at the wrap point (the words of the two scopes
// that DO open the surface, spoken BEFORE any material — the honest
// words travel with the error, the transport never invents its own
// copy); anything else is the honest 500 with the server-side log.
func (s *Server) actionCatalogProblem(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, actionDomain.ErrNotFound):
		s.problem(w, r, http.StatusNotFound, "not_found", "Ресурс не найден.", false, nil)
	case errors.Is(err, actionDomain.ErrForbidden):
		// The app wrap is fmt.Errorf("%w: %s", ErrForbidden, scopeWords):
		// strip the sentinel prefix and speak the scope words themselves.
		message := err.Error()
		if prefix := actionDomain.ErrForbidden.Error() + ": "; strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
		}
		s.problem(w, r, http.StatusForbidden, "actions_forbidden", message, false, nil)
	default:
		s.logger.ErrorContext(r.Context(), "action catalog read failed", "code", "action_catalog_failed", "error", err.Error())
		s.problem(w, r, http.StatusInternalServerError, "internal_error", "Не удалось прочитать каталог действий. Повторите позднее.", true, nil)
	}
}

// BindActionCatalog registers the read-only action catalog surface: the
// machine list, the machine pin read and the catalog page. The service
// is the narrow api.ActionCatalogService (the HandoffService type-assert
// precedent — the app Service carries the compile-time proof).
func (s *Server) BindActionCatalog(service actionAPI.ActionCatalogService) {
	s.actionCatalog = service
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/action-definitions", s.actionCatalogListAPI)
	s.mux.HandleFunc("GET /v1/tenants/{tenant_id}/action-definitions/{definition_id}/{definition_version}", s.actionCatalogGetAPI)
	s.mux.HandleFunc("GET /app/tenants/{tenant_id}/actions", s.actionsPage)
}

// actionCatalogListAPI answers the machine list: the newest version of
// every definition of the served dictionary. The honest empty list is
// the 200 with the empty array (never null) — the empty catalog is a
// state, not a defect (AG-4).
func (s *Server) actionCatalogListAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.actionCatalogScope(w, r)
	if !ok {
		return
	}
	if s.actionCatalog == nil {
		s.actionCatalogProblem(w, r, actionDomain.ErrNotFound)
		return
	}
	defs, err := s.actionCatalog.List(r.Context(), actor, tenant)
	if err != nil {
		s.actionCatalogProblem(w, r, err)
		return
	}
	views := make([]actionDefinitionView, 0, len(defs))
	for _, def := range defs {
		views = append(views, newActionDefinitionView(def))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"action_definitions": views})
}

// actionCatalogGetAPI answers the machine pin read: one exact (id,
// version) pair. The path parameters are decoded strictly — a malformed
// identifier or a non-triple version is the honest 404 of a pin that
// never existed (strict decode before the service; an unknown
// well-formed pin is the service's own 404).
func (s *Server) actionCatalogGetAPI(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.actionCatalogScope(w, r)
	if !ok {
		return
	}
	definitionID, version, decodeOK := decodeActionDefinitionPin(r)
	if !decodeOK {
		s.actionCatalogProblem(w, r, actionDomain.ErrNotFound)
		return
	}
	if s.actionCatalog == nil {
		s.actionCatalogProblem(w, r, actionDomain.ErrNotFound)
		return
	}
	def, err := s.actionCatalog.Get(r.Context(), actor, tenant, definitionID, version)
	if err != nil {
		s.actionCatalogProblem(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"action_definition": newActionDefinitionView(def)})
}

// decodeActionDefinitionPin strictly decodes the two path parameters of
// the pin read. The identifier is 1..120 runes of the id charset; the
// version is the numeric dotted triple X.Y.Z (the parseable-version
// discipline of the domain — the ordering of versions stays total).
func decodeActionDefinitionPin(r *http.Request) (string, string, bool) {
	definitionID := r.PathValue("definition_id")
	version := r.PathValue("definition_version")
	if len([]rune(definitionID)) < 1 || len([]rune(definitionID)) > 120 {
		return "", "", false
	}
	for _, r := range definitionID {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return "", "", false
		}
	}
	if !actionDomain.ParseableVersion(version) {
		return "", "", false
	}
	return definitionID, version, true
}

// ---------------------------------------------------------------------------
// The machine DTO — ActionDefinitionView (the OpenAPI contract schema)
// ---------------------------------------------------------------------------

// actionFieldSchemaView is one declared schema field.
type actionFieldSchemaView struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Description string `json:"description"`
}

// actionPackRefView is the mandatory Domain Pack reference.
type actionPackRefView struct {
	PackID      string `json:"pack_id"`
	PackVersion string `json:"pack_version"`
}

// actionCapabilityRefView is the mandatory connector capability reference.
type actionCapabilityRefView struct {
	ConnectorID      string `json:"connector_id"`
	ConnectorVersion string `json:"connector_version"`
	CapabilityID     string `json:"capability_id"`
}

// actionFactRequirementView is one material dependency of the action.
type actionFactRequirementView struct {
	MaterialField string `json:"material_field"`
	Freshness     string `json:"freshness"`
}

// actionConditionsView is the complete typed condition system of one
// action — every field a dictionary word, every list non-empty.
type actionConditionsView struct {
	RiskClass                string                      `json:"risk_class"`
	AutonomyClass            string                      `json:"autonomy_class"`
	AllowedConversationModes []string                    `json:"allowed_conversation_modes"`
	AllowedRoles             []string                    `json:"allowed_roles"`
	RequiredFacts            []actionFactRequirementView `json:"required_facts"`
	Confirmation             string                      `json:"confirmation"`
	RequiredGates            []string                    `json:"required_gates"`
	TimeoutMS                int                         `json:"timeout_ms"`
	MaterialityClass         string                      `json:"materiality_class"`
	ConsumptionRule          string                      `json:"consumption_rule"`
	AllowedStages            []string                    `json:"allowed_stages"`
}

// actionDeliveryContractView is the delivery classification (AC-3).
type actionDeliveryContractView struct {
	CapabilityClass             string `json:"capability_class"`
	IdempotencyScope            string `json:"idempotency_scope"`
	TerminalEvidence            string `json:"terminal_evidence"`
	ReconciliationQuery         string `json:"reconciliation_query"`
	CancelCompensationSemantics string `json:"cancel_compensation_semantics"`
}

// actionDefinitionView is the read DTO of one action definition — the
// transport maps the domain definition onto this view, never the other
// way around. The field names are the OpenAPI ActionDefinitionView
// contract; the dictionary words are pinned by the OpenAPI contract
// test against the domain dictionaries (drift turns the test red).
type actionDefinitionView struct {
	ID                 string                     `json:"id"`
	Version            string                     `json:"version"`
	Name               string                     `json:"name"`
	Purpose            string                     `json:"purpose"`
	TypedInputSchema   []actionFieldSchemaView    `json:"typed_input_schema"`
	TypedOutputSchema  []actionFieldSchemaView    `json:"typed_output_schema"`
	PackRef            actionPackRefView          `json:"pack_ref"`
	RequiredCapability actionCapabilityRefView    `json:"required_capability"`
	OwningSystem       string                     `json:"owning_system"`
	Conditions         actionConditionsView       `json:"conditions"`
	DeliveryContract   actionDeliveryContractView `json:"delivery_contract"`
}

// newActionDefinitionView maps one domain definition onto the contract
// DTO. The lists are copied onto fresh slices (the response owns its
// material; the empty schema renders as the empty array, never null).
func newActionDefinitionView(def actionDomain.ActionDefinition) actionDefinitionView {
	view := actionDefinitionView{
		ID:      def.ID,
		Version: def.Version,
		Name:    def.Name,
		Purpose: def.Purpose,
		PackRef: actionPackRefView{
			PackID:      def.PackRef.PackID,
			PackVersion: def.PackRef.PackVersion,
		},
		RequiredCapability: actionCapabilityRefView{
			ConnectorID:      def.RequiredCapability.ConnectorID,
			ConnectorVersion: def.RequiredCapability.ConnectorVersion,
			CapabilityID:     def.RequiredCapability.CapabilityID,
		},
		OwningSystem: def.OwningSystem,
		Conditions: actionConditionsView{
			RiskClass:                def.Conditions.RiskClass,
			AutonomyClass:            def.Conditions.AutonomyClass,
			AllowedConversationModes: append([]string{}, def.Conditions.AllowedConversationModes...),
			AllowedRoles:             append([]string{}, def.Conditions.AllowedRoles...),
			Confirmation:             def.Conditions.Confirmation,
			RequiredGates:            append([]string{}, def.Conditions.RequiredGates...),
			TimeoutMS:                def.Conditions.TimeoutMS,
			MaterialityClass:         def.Conditions.MaterialityClass,
			ConsumptionRule:          def.Conditions.ConsumptionRule,
			AllowedStages:            append([]string{}, def.Conditions.AllowedStages...),
		},
		DeliveryContract: actionDeliveryContractView{
			CapabilityClass:             def.DeliveryContract.CapabilityClass,
			IdempotencyScope:            def.DeliveryContract.IdempotencyScope,
			TerminalEvidence:            def.DeliveryContract.TerminalEvidence,
			ReconciliationQuery:         def.DeliveryContract.ReconciliationQuery,
			CancelCompensationSemantics: def.DeliveryContract.CancelCompensationSem,
		},
	}
	for _, field := range def.TypedInputSchema {
		view.TypedInputSchema = append(view.TypedInputSchema, actionFieldSchemaView{
			Name: field.Name, Label: field.Label, Type: string(field.Type),
			Required: field.Required, Secret: field.Secret, Description: field.Description,
		})
	}
	for _, field := range def.TypedOutputSchema {
		view.TypedOutputSchema = append(view.TypedOutputSchema, actionFieldSchemaView{
			Name: field.Name, Label: field.Label, Type: string(field.Type),
			Required: field.Required, Secret: field.Secret, Description: field.Description,
		})
	}
	for _, fact := range def.Conditions.RequiredFacts {
		view.Conditions.RequiredFacts = append(view.Conditions.RequiredFacts, actionFactRequirementView{
			MaterialField: fact.MaterialField, Freshness: fact.Freshness,
		})
	}
	if view.TypedInputSchema == nil {
		view.TypedInputSchema = []actionFieldSchemaView{}
	}
	if view.TypedOutputSchema == nil {
		view.TypedOutputSchema = []actionFieldSchemaView{}
	}
	if view.Conditions.RequiredFacts == nil {
		view.Conditions.RequiredFacts = []actionFactRequirementView{}
	}
	return view
}

// ---------------------------------------------------------------------------
// The «Каталог действий» page
// ---------------------------------------------------------------------------

// actionCardView is the flattened card projection of one definition:
// the headline words render textually (name, version, risk, stages,
// capability, owning system, the human-only state of unsafe); the full
// condition system and the delivery contract live behind the progressive
// disclosure — the card never floods the reader with every word at once.
type actionCardView struct {
	ID                      string
	Version                 string
	Name                    string
	Purpose                 string
	RiskClass               string
	StagesText              string
	ModesText               string
	RolesText               string
	FactsText               string
	GatesText               string
	TimeoutText             string
	Autonomy                string
	Materiality             string
	Consumption             string
	Confirmation            string
	OwningSystem            string
	PackText                string
	CapabilityText          string
	DeliveryClass           string
	IdempotencyScopeText    string
	TerminalEvidenceText    string
	ReconciliationQueryText string
	CancelCompensationText  string
	Unsafe                  bool
	HasInputFields          bool
	InputFields             []actionFieldRow
	HasOutputFields         bool
	OutputFields            []actionFieldRow
}

// actionFieldRow is one declared schema field of the card disclosure.
type actionFieldRow struct {
	Name        string
	Label       string
	Type        string
	Required    string
	Secret      string
	Description string
}

// actionCatalogPageView carries the page material: the tenant, the cards
// of the newest definitions and the honest empty state (the reason
// words — never a bare stub page).
type actionCatalogPageView struct {
	TenantID  string
	HasCards  bool
	Cards     []actionCardView
	EmptyText string
}

// actionCatalogEmptyText is the honest reason of the empty catalog
// (AG-4): the words explain WHY the catalog is empty — the definitions
// appear only with published pack connectors — so the absence is a
// readable state, not a silent blank (AC-11).
const actionCatalogEmptyText = "Каталог действий пуст: определения появятся, когда опубликованный Domain Pack будет связан с коннектором, объявляющим query/command-способности, а полные ссылки определения пройдут проверку допуска. Сегодня ни один пак не связан с capability-коннектором — это честное состояние словаря, а не ошибка."

// actionsPage renders the «Каталог действий» page: the cards of the
// newest definition versions with the progressive disclosure of the
// condition system (details/summary — the provider-technique
// precedent), or the honest empty state with the reason words.
func (s *Server) actionsPage(w http.ResponseWriter, r *http.Request) {
	actor, tenant, ok := s.actionCatalogScope(w, r)
	if !ok {
		return
	}
	if s.actionCatalog == nil {
		s.actionCatalogProblem(w, r, actionDomain.ErrNotFound)
		return
	}
	defs, err := s.actionCatalog.List(r.Context(), actor, tenant)
	if err != nil {
		s.actionCatalogProblem(w, r, err)
		return
	}
	data := actionCatalogPageView{TenantID: tenant.String()}
	if len(defs) > 0 {
		data.HasCards = true
		for _, def := range defs {
			data.Cards = append(data.Cards, newActionCardView(def))
		}
	} else {
		data.EmptyText = actionCatalogEmptyText
	}
	s.render(w, "actions", data)
}

// newActionCardView flattens one definition onto the card projection.
// Every condition word renders textually; the human-only discipline of
// unsafe is stated on the card (AC-3: unsafe never leaves the human
// loop).
func newActionCardView(def actionDomain.ActionDefinition) actionCardView {
	card := actionCardView{
		ID:           def.ID,
		Version:      def.Version,
		Name:         def.Name,
		Purpose:      def.Purpose,
		RiskClass:    def.Conditions.RiskClass,
		StagesText:   strings.Join(def.Conditions.AllowedStages, "; "),
		ModesText:    strings.Join(def.Conditions.AllowedConversationModes, "; "),
		RolesText:    strings.Join(def.Conditions.AllowedRoles, "; "),
		GatesText:    strings.Join(def.Conditions.RequiredGates, "; "),
		TimeoutText:  formatActionTimeout(def.Conditions.TimeoutMS),
		Autonomy:     def.Conditions.AutonomyClass,
		Materiality:  def.Conditions.MaterialityClass,
		Consumption:  def.Conditions.ConsumptionRule,
		Confirmation: def.Conditions.Confirmation,
		OwningSystem: def.OwningSystem,
		PackText:     def.PackRef.PackID + "@" + def.PackRef.PackVersion,
		CapabilityText: def.RequiredCapability.ConnectorID + "@" +
			def.RequiredCapability.ConnectorVersion + "/" + def.RequiredCapability.CapabilityID,
		DeliveryClass:           def.DeliveryContract.CapabilityClass,
		IdempotencyScopeText:    def.DeliveryContract.IdempotencyScope,
		TerminalEvidenceText:    def.DeliveryContract.TerminalEvidence,
		ReconciliationQueryText: def.DeliveryContract.ReconciliationQuery,
		CancelCompensationText:  def.DeliveryContract.CancelCompensationSem,
		Unsafe:                  def.DeliveryContract.CapabilityClass == actionDomain.CapabilityUnsafe,
	}
	facts := make([]string, 0, len(def.Conditions.RequiredFacts))
	for _, fact := range def.Conditions.RequiredFacts {
		facts = append(facts, fact.MaterialField+" ("+fact.Freshness+")")
	}
	card.FactsText = strings.Join(facts, "; ")
	card.HasInputFields = len(def.TypedInputSchema) > 0
	card.HasOutputFields = len(def.TypedOutputSchema) > 0
	card.InputFields = actionFieldRows(def.TypedInputSchema)
	card.OutputFields = actionFieldRows(def.TypedOutputSchema)
	return card
}

// actionFieldRows maps the declared schema fields onto the card rows.
func actionFieldRows(fields []actionDomain.FieldSchema) []actionFieldRow {
	rows := make([]actionFieldRow, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, actionFieldRow{
			Name:        field.Name,
			Label:       field.Label,
			Type:        string(field.Type),
			Required:    actionYesNo(field.Required),
			Secret:      actionYesNo(field.Secret),
			Description: field.Description,
		})
	}
	return rows
}

// actionYesNo renders the honest yes/no word of the boolean card words.
func actionYesNo(value bool) string {
	if value {
		return "да"
	}
	return "нет"
}

// formatActionTimeout renders the execution bound of one action in the
// honest ms words.
func formatActionTimeout(timeoutMS int) string {
	return strconv.Itoa(timeoutMS) + " мс"
}
