package catalog

import (
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/connector/domain"
)

// The cross-module mirror of spec 4-1: the employee-domain pilot route
// dictionary and the connector catalog declarations pin each other
// (test-only import — no production dependency edge; the production read
// path stays the narrow ConnectorFacts port).
import employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"

func TestCatalogListsEveryPublishedDefinition(t *testing.T) {
	defs := List()
	if len(defs) < 7 {
		t.Fatalf("expected at least 7 definitions, got %d", len(defs))
	}
	ids := map[string]bool{}
	for _, d := range defs {
		ids[d.ID] = true
	}
	for _, want := range []string{"staffy-synthetic-telephony", "novofon-sip", "mango-office-sip", "jivo-chat", "abcp-parts", "yandex-speechkit", "exolve-sip", "montagio-web", "partner-intake", "b2b-intake"} {
		if !ids[want] {
			t.Errorf("definition %s missing", want)
		}
	}
}

func TestCatalogOnlySyntheticConnectorIsConnectable(t *testing.T) {
	// Story 7-4: the external MCP class joins the connectable set — the
	// client machine and the probe router are REAL production code.
	for _, d := range List() {
		if d.ID == "staffy-synthetic-telephony" || d.ID == "staffy-external-mcp-server" {
			if !d.Connectable() {
				t.Fatalf("%s must be connectable", d.ID)
			}
			continue
		}
		if d.Connectable() {
			t.Errorf("%s must not be connectable", d.ID)
		}
	}
}

func TestCatalogStatusesAreTextOnly(t *testing.T) {
	valid := map[domain.SupportStatus]bool{domain.StatusSupported: true, domain.StatusPilot: true, domain.StatusCandidate: true, domain.StatusDeferred: true, domain.StatusUnavailable: true}
	seen := map[domain.SupportStatus]bool{}
	for _, d := range List() {
		if !valid[d.SupportStatus] {
			t.Errorf("%s has invalid status %q", d.ID, d.SupportStatus)
		}
		seen[d.SupportStatus] = true
	}
	if !seen[domain.StatusSupported] || !seen[domain.StatusCandidate] || !seen[domain.StatusDeferred] || !seen[domain.StatusUnavailable] {
		t.Fatalf("catalog must demonstrate the status vocabulary, got %v", seen)
	}
}

func TestNovofonPriorityNoteHiddenWithoutAdapter(t *testing.T) {
	def, ok := Get("novofon-sip", "1.0.0")
	if !ok {
		t.Fatal("novofon definition missing")
	}
	if def.AdapterImplemented {
		t.Fatal("novofon adapter must not be implemented in this slice")
	}
	if def.Connectable() {
		t.Fatal("novofon must not be connectable")
	}
	if def.SupportStatus != domain.StatusCandidate {
		t.Fatalf("novofon status = %s", def.SupportStatus)
	}
}

func TestExolveIsUnavailableAndNeverConnectable(t *testing.T) {
	def, ok := Get("exolve-sip", "1.0.0")
	if !ok {
		t.Fatal("exolve definition missing")
	}
	if def.SupportStatus != domain.StatusUnavailable {
		t.Fatalf("exolve status = %s", def.SupportStatus)
	}
	if def.Connectable() {
		t.Fatal("unavailable definition must never be connectable")
	}
}

func TestGetPinsExactVersion(t *testing.T) {
	if _, ok := Get("staffy-synthetic-telephony", "1.0.0"); !ok {
		t.Fatal("pinned version missing")
	}
	if _, ok := Get("staffy-synthetic-telephony", "9.9.9"); ok {
		t.Fatal("unknown version must not resolve")
	}
}

func TestListReturnsDefensiveCopies(t *testing.T) {
	first := List()
	first[0].Fields[0].Name = "mutated"
	second := List()
	if second[0].Fields[0].Name == "mutated" {
		t.Fatal("List must return defensive copies")
	}
}

func TestGroupByTypeCoversFixedOrder(t *testing.T) {
	grouped := GroupByType(List())
	if _, ok := grouped[domain.TypeStorage]; !ok {
		t.Fatal("storage group missing")
	}
	if len(grouped[domain.TypeSIP]) < 3 {
		t.Fatalf("expected at least 3 SIP definitions, got %d", len(grouped[domain.TypeSIP]))
	}
}

// --- Spec 4-1 (Pilot Admission Registry): the three declared versions ------

func specJivoChat(t *testing.T) domain.ConnectorDefinition {
	def, ok := Get("jivo-chat", "1.1.0")
	if !ok {
		t.Fatal("jivo-chat 1.1.0 must be published (spec 4-1)")
	}
	return def
}

func specAbcpParts(t *testing.T) domain.ConnectorDefinition {
	def, ok := Get("abcp-parts", "1.1.0")
	if !ok {
		t.Fatal("abcp-parts 1.1.0 must be published (spec 4-1)")
	}
	return def
}

func specSyntheticTelephony(t *testing.T) domain.ConnectorDefinition {
	def, ok := Get("staffy-synthetic-telephony", "1.2.0")
	if !ok {
		t.Fatal("staffy-synthetic-telephony 1.2.0 must be published (spec 4-1)")
	}
	return def
}

func TestJivoChatDeclaredVersionCarriesTheEightCapabilities(t *testing.T) {
	def := specJivoChat(t)
	if def.AdapterImplemented || def.Connectable() {
		t.Fatal("the Jivo adapter is honestly not implemented (AG-4): the card is not connectable")
	}
	if def.SupportStatus != domain.StatusCandidate {
		t.Fatalf("jivo-chat 1.1.0 is a declared candidate, got %s", def.SupportStatus)
	}
	want := []string{
		"event.inbound-message", "command.outbound-reply", "query.delivery-status", "command.assign-to-human",
		"event.acknowledgement", "query.one-writer-state", "query.media-upload-support", "query.fallback-state",
	}
	if len(def.Capabilities) != 8 {
		t.Fatalf("jivo-chat 1.1.0 declares exactly 8 capabilities (AC-3), got %d", len(def.Capabilities))
	}
	for index, id := range want {
		if def.Capabilities[index].ID != id {
			t.Fatalf("capability %d = %s, want %s", index, def.Capabilities[index].ID, id)
		}
		if !def.Capabilities[index].Complete() {
			t.Fatalf("%s must carry the full operational semantics (1-9 discipline)", id)
		}
	}
}

func TestAbcpPartsDeclaredVersionCarriesSixQueriesAndTheFencedOrder(t *testing.T) {
	def := specAbcpParts(t)
	if def.AdapterImplemented || def.Connectable() {
		t.Fatal("the ABCP adapter is honestly not implemented (AG-4)")
	}
	want := []string{
		"query.identify-vehicle", "query.catalog-navigation", "query.fitment-applicability",
		"query.oem-cross", "query.supplier-offers", "query.price-availability-term",
	}
	var queries []string
	for _, capability := range def.Capabilities {
		if capability.Operation == domain.OperationQuery {
			queries = append(queries, capability.ID)
		}
		if !capability.Complete() {
			t.Fatalf("%s must carry the full operational semantics", capability.ID)
		}
	}
	if len(queries) != 6 {
		t.Fatalf("abcp-parts 1.1.0 declares exactly 6 read-only queries (AC-7), got %v", queries)
	}
	for index, id := range want {
		if queries[index] != id {
			t.Fatalf("query %d = %s, want %s", index, queries[index], id)
		}
	}
	order, ok := func() (domain.CapabilityDeclaration, bool) {
		for _, capability := range def.Capabilities {
			if capability.ID == "command.prepare-client-order" {
				return capability, true
			}
		}
		return domain.CapabilityDeclaration{}, false
	}()
	if !ok {
		t.Fatal("command.prepare-client-order must be declared (AC-8)")
	}
	if order.ProbeAllowed {
		t.Fatal("the mutating capability is probe-forbidden (AC-8: ProbeAllowed false)")
	}
	if order.Reconciliation != "query_reconcilable по GATE-REGISTRY OQ-3: reread статуса prepared order; fence — явное одобрение менеджера" {
		t.Fatalf("the order reconciliation carries the OQ-3 fence word, got %q", order.Reconciliation)
	}
	if !order.Idempotent {
		t.Fatal("prepare-client-order is idempotent by the typed key (AC-8)")
	}
	if len(def.ManagerOnlyActions) != 1 || def.ManagerOnlyActions[0] != "поиск/заказ через интерфейс менеджера ABCP — не API capability" {
		t.Fatalf("the manager-only dictionary word (AC-7 tail): %+v", def.ManagerOnlyActions)
	}
}

func TestSyntheticTelephonyDeclaredVersionAddsFiveSIPQueries(t *testing.T) {
	def := specSyntheticTelephony(t)
	if !def.AdapterImplemented || !def.Connectable() {
		t.Fatal("the synthetic telephony adapter is implemented — the only live probe contour")
	}
	newQueries := []string{"query.transfer-bridge", "query.callback-state", "query.dtmf-support", "query.playback-delivery", "query.failure-states"}
	ids := map[string]bool{}
	for _, capability := range def.Capabilities {
		ids[capability.ID] = true
		if !capability.Complete() {
			t.Fatalf("%s must carry the full operational semantics", capability.ID)
		}
	}
	if len(def.Capabilities) != 8 {
		t.Fatalf("1.2.0 carries the 3 prior + 5 new SIP queries, got %d", len(def.Capabilities))
	}
	for _, id := range newQueries {
		if !ids[id] {
			t.Fatalf("SIP query %s missing from 1.2.0 (AC-6)", id)
		}
	}
}

func TestOldVersionsStayImmutableAndUnoffered(t *testing.T) {
	// The immutability of pins: older versions resolve unchanged and are
	// never offered as new.
	jivo, ok := Get("jivo-chat", "1.0.0")
	if !ok || len(jivo.Capabilities) != 0 || jivo.SupportStatus != domain.StatusDeferred {
		t.Fatalf("jivo-chat 1.0.0 stays the empty deferred pin, got %+v", jivo)
	}
	abcp, ok := Get("abcp-parts", "1.0.0")
	if !ok || len(abcp.Capabilities) != 0 || abcp.SupportStatus != domain.StatusDeferred {
		t.Fatalf("abcp-parts 1.0.0 stays the empty deferred pin, got %+v", abcp)
	}
	prior, ok := Get("staffy-synthetic-telephony", "1.1.0")
	if !ok || len(prior.Capabilities) != 3 {
		t.Fatalf("synthetic telephony 1.1.0 stays the 3-capability pin, got %d capabilities", len(prior.Capabilities))
	}
	oldest, ok := Get("staffy-synthetic-telephony", "1.0.0")
	if !ok || len(oldest.Capabilities) != 0 {
		t.Fatalf("synthetic telephony 1.0.0 stays the empty pin, got %d capabilities", len(oldest.Capabilities))
	}
	novofon, ok := Get("novofon-sip", "1.0.0")
	if !ok || len(novofon.Capabilities) != 0 || novofon.SupportStatus != domain.StatusCandidate {
		t.Fatalf("novofon-sip 1.0.0 stays the empty candidate pin (immutability 1-8), got %+v", novofon)
	}
	for _, offered := range List() {
		switch offered.ID {
		case "jivo-chat":
			if offered.Version != "1.1.0" {
				t.Fatalf("jivo-chat offers the newest contract, got %s", offered.Version)
			}
		case "abcp-parts":
			if offered.Version != "1.1.0" {
				t.Fatalf("abcp-parts offers the newest contract, got %s", offered.Version)
			}
		case "staffy-synthetic-telephony":
			if offered.Version != "1.2.0" {
				t.Fatalf("synthetic telephony offers the newest contract, got %s", offered.Version)
			}
		case "novofon-sip":
			if offered.Version != "1.1.0" {
				t.Fatalf("novofon-sip offers the newest contract, got %s", offered.Version)
			}
		}
	}
	if _, current, _ := Latest("jivo-chat", "1.0.0"); current {
		t.Fatal("an older pin never counts as the current contract")
	}
}

func TestProbeAllowedOnlyOnReadOnlyQueries(t *testing.T) {
	// The 1-9 discipline across the whole catalog: only read-only query
	// capabilities may be probe-verified; commands and events never.
	for _, def := range append([]domain.ConnectorDefinition{specJivoChat(t), specAbcpParts(t)}, specSyntheticTelephony(t)) {
		for _, capability := range def.Capabilities {
			if capability.ProbeAllowed && capability.Operation != domain.OperationQuery {
				t.Fatalf("%s of %s@%s: ProbeAllowed is only for read-only queries", capability.ID, def.ID, def.Version)
			}
		}
	}
}

func TestSupplierFenceWordsAreAbsentFromTheCatalog(t *testing.T) {
	// The Fixed Oleg pilot fence: supplier_form/send never appears in the
	// dictionaries — no capability performs the supplier send.
	for _, def := range List() {
		if strings.Contains(def.Purpose, "supplier_form") {
			t.Fatalf("%s purpose carries the fenced word", def.ID)
		}
		for _, capability := range def.Capabilities {
			if strings.Contains(capability.ID, "supplier_form") || strings.Contains(capability.Description, "supplier_form") {
				t.Fatalf("%s of %s carries the fenced word", capability.ID, def.ID)
			}
		}
		for _, action := range def.ManagerOnlyActions {
			if strings.Contains(action, "supplier_form") {
				t.Fatalf("manager action of %s carries the fenced word", def.ID)
			}
		}
	}
}

func TestCatalogMirrorsThePilotRouteDictionary(t *testing.T) {
	// Spec 4-1 acceptance: the declarations are mirror-equal to the domain
	// route dictionary — every required capability of every pilot route
	// resolves to a declared capability of the LATEST version of its
	// connector, and the required pack pin matches the catalog pack words.
	latest := map[string]map[string]bool{}
	for _, def := range List() {
		latest[def.ID] = map[string]bool{}
		for _, capability := range def.Capabilities {
			latest[def.ID][capability.ID] = true
		}
	}
	for _, route := range employeeDomain.PilotRoutes() {
		for _, requirement := range route.RequiredCapabilities {
			declared, ok := latest[requirement.ConnectorID][requirement.ID]
			if !ok || !declared {
				t.Fatalf("route %s requires %s of %s — not declared by the latest catalog version (mirror drift)",
					route.ID, requirement.ID, requirement.ConnectorID)
			}
		}
	}
	// The voice route consumes the synthetic telephony contract: its six
	// requirements are the newest 1.2.0 query set.
	voice := latest["staffy-synthetic-telephony"]
	for _, id := range []string{"query.trunk-status", "query.transfer-bridge", "query.callback-state", "query.dtmf-support", "query.playback-delivery", "query.failure-states"} {
		if !voice[id] {
			t.Fatalf("voice_sip requirement %s missing from staffy-synthetic-telephony@1.2.0", id)
		}
	}
}

// --- Spec 5-1 (Montagio Admission Registry): the four declared versions ---

func specNovofonSip(t *testing.T) domain.ConnectorDefinition {
	def, ok := Get("novofon-sip", "1.1.0")
	if !ok {
		t.Fatal("novofon-sip 1.1.0 must be published (spec 5-1)")
	}
	return def
}

func specMontagioWeb(t *testing.T) domain.ConnectorDefinition {
	def, ok := Get("montagio-web", "1.0.0")
	if !ok {
		t.Fatal("montagio-web 1.0.0 must be published (spec 5-1)")
	}
	return def
}

func specPartnerIntake(t *testing.T) domain.ConnectorDefinition {
	def, ok := Get("partner-intake", "1.0.0")
	if !ok {
		t.Fatal("partner-intake 1.0.0 must be published (spec 5-1)")
	}
	return def
}

func specB2BIntake(t *testing.T) domain.ConnectorDefinition {
	def, ok := Get("b2b-intake", "1.0.0")
	if !ok {
		t.Fatal("b2b-intake 1.0.0 must be published (spec 5-1)")
	}
	return def
}

func TestNovofonSipDeclaredVersionCarriesTheTwelveQueries(t *testing.T) {
	def := specNovofonSip(t)
	if def.AdapterImplemented || def.Connectable() {
		t.Fatal("the Novofon adapter is honestly not implemented (AG-4): the general documentation is never a proof of the concrete tariff and account")
	}
	if def.SupportStatus != domain.StatusCandidate {
		t.Fatalf("novofon-sip 1.1.0 is a declared candidate, got %s", def.SupportStatus)
	}
	if def.RequiredAccountType != "novofon_business" {
		t.Fatalf("novofon-sip 1.1.0 pins the account scope, got %s", def.RequiredAccountType)
	}
	want := []string{
		"query.inbound-calls", "query.outbound-calls", "query.media-codec", "query.streaming",
		"query.dtmf-support", "query.transfer-bridge", "query.callback-state", "query.caller-id",
		"query.recording", "query.webhooks", "query.delivery-states", "query.failure-states",
	}
	if len(def.Capabilities) != 12 {
		t.Fatalf("novofon-sip 1.1.0 declares exactly 12 read-only queries (AC-3), got %d", len(def.Capabilities))
	}
	for index, id := range want {
		capability := def.Capabilities[index]
		if capability.ID != id {
			t.Fatalf("capability %d = %s, want %s", index, capability.ID, id)
		}
		if !capability.Complete() {
			t.Fatalf("%s must carry the full operational semantics (1-9 discipline)", id)
		}
		if capability.Operation != domain.OperationQuery || !capability.ProbeAllowed {
			t.Fatalf("%s is a read-only probe-allowed query (AC-3)", id)
		}
	}
}

func TestMontagioWebDeclaredVersionCarriesTheNineCapabilities(t *testing.T) {
	def := specMontagioWeb(t)
	if def.AdapterImplemented || def.Connectable() {
		t.Fatal("the Montagio site adapter is honestly not implemented (AG-4)")
	}
	if def.Type != domain.TypeChat {
		t.Fatalf("montagio-web is a chat surface, got %s", def.Type)
	}
	want := []string{
		"query.contract-schema", "query.authentication", "event.inbound-submission", "command.outbound-reply",
		"query.correlation-state", "query.media-upload-support", "query.delivery-status", "query.rate-limits",
		"query.failure-states",
	}
	if len(def.Capabilities) != 9 {
		t.Fatalf("montagio-web 1.0.0 declares exactly 9 capabilities (AC-5), got %d", len(def.Capabilities))
	}
	for index, id := range want {
		if def.Capabilities[index].ID != id {
			t.Fatalf("capability %d = %s, want %s", index, def.Capabilities[index].ID, id)
		}
		if !def.Capabilities[index].Complete() {
			t.Fatalf("%s must carry the full operational semantics (1-9 discipline)", id)
		}
	}
	reply := def.Capabilities[3]
	if reply.Operation != domain.OperationCommand || reply.ProbeAllowed {
		t.Fatal("command.outbound-reply is the declared mutating reply — probe-forbidden until the Action Gateway (AG-4)")
	}
	if !strings.Contains(def.Purpose, "неполная заявка не превращается") {
		t.Fatalf("the AC-6 fence word travels with the card purpose, got %q", def.Purpose)
	}
}

func TestPartnerIntakeDeclaredVersionCarriesTheFiveCapabilities(t *testing.T) {
	def := specPartnerIntake(t)
	if def.AdapterImplemented || def.Connectable() {
		t.Fatal("the partner intake adapter is honestly not implemented (AG-4)")
	}
	want := []string{
		"event.inbound-partner-lead", "query.lead-contract-version", "query.consent-evidence",
		"query.delivery-status", "query.failure-states",
	}
	if len(def.Capabilities) != 5 {
		t.Fatalf("partner-intake 1.0.0 declares exactly 5 capabilities (AC-6), got %d", len(def.Capabilities))
	}
	for index, id := range want {
		if def.Capabilities[index].ID != id {
			t.Fatalf("capability %d = %s, want %s", index, def.Capabilities[index].ID, id)
		}
		if !def.Capabilities[index].Complete() {
			t.Fatalf("%s must carry the full operational semantics", id)
		}
	}
}

func TestB2BIntakeDeclaredVersionCarriesTheFiveCapabilities(t *testing.T) {
	def := specB2BIntake(t)
	if def.AdapterImplemented || def.Connectable() {
		t.Fatal("the B2B intake adapter is honestly not implemented (AG-4)")
	}
	want := []string{
		"event.inbound-b2b-request", "query.organization-fields", "query.authority-evidence",
		"query.delivery-status", "query.failure-states",
	}
	if len(def.Capabilities) != 5 {
		t.Fatalf("b2b-intake 1.0.0 declares exactly 5 capabilities (AC-7), got %d", len(def.Capabilities))
	}
	for index, id := range want {
		if def.Capabilities[index].ID != id {
			t.Fatalf("capability %d = %s, want %s", index, def.Capabilities[index].ID, id)
		}
		if !def.Capabilities[index].Complete() {
			t.Fatalf("%s must carry the full operational semantics", id)
		}
	}
	authority := def.Capabilities[2]
	if !strings.Contains(authority.Description, "unknown, не подразумевается") {
		t.Fatalf("the AC-7 authority fence travels with the capability, got %q", authority.Description)
	}
}

func TestNoAvitoDeclarationsExist(t *testing.T) {
	// AC-8: Avito stays outside the pilot production substrate — declaring a
	// capability of an unproven provider would contradict the honest
	// dictionary, so no Avito connector exists in the catalog at all.
	for _, def := range List() {
		if strings.Contains(strings.ToLower(def.ID), "avito") || strings.Contains(strings.ToLower(def.Name), "avito") {
			t.Fatalf("no Avito declaration may exist in the catalog, got %s", def.ID)
		}
	}
	if _, ok := Get("avito", "1.0.0"); ok {
		t.Fatal("no Avito definition resolves")
	}
}

func TestMontagioDeclarationsOfferNoExecutableMutations(t *testing.T) {
	// The honesty resolution of the spec's Never clause («мутирующих
	// capabilities новых НЕТ»): the one declared command word
	// (command.outbound-reply — the existing 4-1 dictionary word, AC-5) and
	// every event stay structurally unexecutable — ProbeAllowed false and the
	// adapter honestly not implemented; no probe and no connection can ever
	// exercise them this slice.
	for _, def := range []domain.ConnectorDefinition{specNovofonSip(t), specMontagioWeb(t), specPartnerIntake(t), specB2BIntake(t)} {
		if def.AdapterImplemented {
			t.Fatalf("%s@%s: the adapter must stay not implemented (AG-4)", def.ID, def.Version)
		}
		for _, capability := range def.Capabilities {
			if capability.ProbeAllowed && capability.Operation != domain.OperationQuery {
				t.Fatalf("%s of %s: ProbeAllowed is only for read-only queries", capability.ID, def.ID)
			}
			if capability.Operation == domain.OperationCommand && capability.ProbeAllowed {
				t.Fatalf("%s of %s: a command is never probe-allowed", capability.ID, def.ID)
			}
		}
	}
	// The only command word of the slice is the existing 4-1 dictionary word.
	commands := map[string]bool{}
	for _, def := range []domain.ConnectorDefinition{specNovofonSip(t), specMontagioWeb(t), specPartnerIntake(t), specB2BIntake(t)} {
		for _, capability := range def.Capabilities {
			if capability.Operation == domain.OperationCommand {
				commands[capability.ID] = true
			}
		}
	}
	if len(commands) != 1 || !commands["command.outbound-reply"] {
		t.Fatalf("the slice carries exactly the one existing command word (command.outbound-reply), got %v", commands)
	}
}

func TestCatalogMirrorsTheMontagioRouteDictionary(t *testing.T) {
	// Spec 5-1 acceptance: the declarations are mirror-equal to the Montagio
	// domain route dictionary — every required capability of every Montagio
	// route resolves to a declared capability of the LATEST version of its
	// connector.
	latest := map[string]map[string]bool{}
	for _, def := range List() {
		latest[def.ID] = map[string]bool{}
		for _, capability := range def.Capabilities {
			latest[def.ID][capability.ID] = true
		}
	}
	for _, route := range employeeDomain.MontagioRoutes() {
		for _, requirement := range route.RequiredCapabilities {
			declared, ok := latest[requirement.ConnectorID][requirement.ID]
			if !ok || !declared {
				t.Fatalf("montagio route %s requires %s of %s — not declared by the latest catalog version (mirror drift)",
					route.ID, requirement.ID, requirement.ConnectorID)
			}
		}
	}
	// The reverse mirror: the four Montagio connectors carry exactly the
	// route dictionary requirements — no undeclared surplus capabilities.
	required := map[string]map[string]bool{}
	for _, route := range employeeDomain.MontagioRoutes() {
		for _, requirement := range route.RequiredCapabilities {
			if required[requirement.ConnectorID] == nil {
				required[requirement.ConnectorID] = map[string]bool{}
			}
			required[requirement.ConnectorID][requirement.ID] = true
		}
	}
	for _, connectorID := range []string{"novofon-sip", "montagio-web", "partner-intake", "b2b-intake"} {
		def, ok := Get(connectorID, latestVersionOf(t, connectorID))
		if !ok {
			t.Fatalf("%s missing from the catalog", connectorID)
		}
		for _, capability := range def.Capabilities {
			if !required[connectorID][capability.ID] {
				t.Fatalf("%s@%s declares %s — not required by any Montagio route (surplus declaration)", connectorID, def.Version, capability.ID)
			}
		}
	}
}

func latestVersionOf(t *testing.T, connectorID string) string {
	t.Helper()
	def, _, found := Latest(connectorID, "")
	if !found {
		t.Fatalf("connector %s not found", connectorID)
	}
	return def.Version
}
