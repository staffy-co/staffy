package httptransport

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.10 servicing wire shapes: the panel rendering (the request
// feed with the proof pins, the TYPED identifier with the kind words,
// the lookup outcome word, the disclosure state TEXT, the LIVE client
// words and the four cancel states), the record form (the intent
// select of the 8 words, the kind select of the 2 words, the value),
// the stable word-links of the human contours and the OpenAPI contract
// mirrors with the fence probes (no supplier-send field — OQ-3; no
// notification delivery field — OQ-1/OQ-2; no price field — the
// purchasing data stays hidden; no phone/email lookup kind — OQ-16).
// ---------------------------------------------------------------------------

// The fixture identities of the servicing wire world.
var (
	servicingWireTenant = id.MustParse("01994f64-6c00-7000-8000-000000000003")
	servicingWireCase   = id.MustParse("01994f64-6c00-7000-8000-00000000aa03")
	servicingWireOper   = id.MustParse("01994f64-6c00-7000-8000-00000000ee03")
	servicingWireActor  = id.MustParse("01994f64-6c00-7000-8000-00000000b203")
)

// fakeOrderServicingSurface is the scriptable OrderServicingService of
// the wire tests: the read answers ride the fields, the record command
// answers the scripted outcome (the command ladder is pinned by the
// app tests; here the wire shape and the route ladder ride).
type fakeOrderServicingSurface struct {
	view     actionDomain.OrderServicingView
	readErr  error
	recorded []actionAPI.RecordOrderServicingRequestCommand
	outcome  actionAPI.OrderServicingOutcome
	recErr   error
}

func (f *fakeOrderServicingSurface) RecordOrderServicingRequest(_ context.Context, command actionAPI.RecordOrderServicingRequestCommand) (actionAPI.OrderServicingOutcome, error) {
	f.recorded = append(f.recorded, command)
	return f.outcome, f.recErr
}

func (f *fakeOrderServicingSurface) ReadOrderServicingView(context.Context, identityDomain.Principal, id.UUID, id.UUID, time.Time) (actionDomain.OrderServicingView, error) {
	return f.view, f.readErr
}

// newOrderServicingWireServer wires the server of the wire tests over
// the scriptable servicing surface.
func newOrderServicingWireServer(t *testing.T, surface *fakeOrderServicingSurface) *Server {
	t.Helper()
	identity := fakeCorrectionWireIdentity{fakeIdentity: fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		}}}
	server, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.orderServicing = surface
	return server
}

// servicingWireView builds the full servicing view fixture of the wire
// tests: the verified unique-match request with the LIVE client words
// and the sources BY NAME, plus the denied request with the gate word
// (the AC-1 leak probe of the RENDERED page) and the cancel request
// with the four states.
func servicingWireView() actionDomain.OrderServicingView {
	issued := time.Date(2026, 10, 25, 11, 50, 0, 0, time.UTC)
	expires := issued.Add(time.Hour)
	return actionDomain.OrderServicingView{
		PhoneBoundaryWord:     actionDomain.ServicingPhoneSearchBoundaryWord,
		ConnectorBoundaryWord: actionDomain.ServicingConnectorBoundaryWord,
		NoDeliveryWord:        actionDomain.ServicingNoDeliveryWord,
		RequestRecordedWord:   actionDomain.ServicingRequestRecordedWord,
		Requests: []actionDomain.OrderServicingRequestView{
			{
				Request: actionDomain.OrderServicingRequest{
					RequestNo: 1, Intent: actionDomain.ServicingIntentOrderStatus,
					IdentifierKind:      actionDomain.LookupKindABCPClientOrderID,
					IdentifierValue:     "ABCP-ORDER-66",
					ResolvedOperationID: servicingWireOper,
					LookupOutcome:       actionDomain.LookupOutcomeUnique,
					ProofState:          actionDomain.ProofStateVerified,
					ProofMethod:         "known_number_callback",
					ProofIssuedAt:       issued, ProofExpiresAt: expires,
					Disclosure: actionDomain.DisclosureAllowed,
					ActorID:    servicingWireActor, RecordedAt: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC),
				},
				IntentShortWord: actionDomain.ServicingIntentShortWords[actionDomain.ServicingIntentOrderStatus],
				KindShortWord:   actionDomain.LookupKindShortWords[actionDomain.LookupKindABCPClientOrderID],
				KindSourceWord:  actionDomain.LookupKindSourceWords[actionDomain.LookupKindABCPClientOrderID],
				OutcomeWord:     actionDomain.LookupOutcomeWords[actionDomain.LookupOutcomeUnique],
				Sources:         []string{actionDomain.OrderFactSourceSystemReceipt, actionDomain.OrderFactSourceManagerRecord},
				ClientWords:     []string{actionDomain.OrderStageSupplierSentWord, actionDomain.OrderClientBoundaryWord, actionDomain.ServicingIntentOrderStatusWord},
			},
			{
				Request: actionDomain.OrderServicingRequest{
					RequestNo: 2, Intent: actionDomain.ServicingIntentCancel,
					IdentifierKind:      actionDomain.LookupKindSupplierOrderID,
					IdentifierValue:     "SUP-66",
					LookupOutcome:       actionDomain.LookupOutcomeUnique,
					ResolvedOperationID: servicingWireOper,
					ProofState:          actionDomain.ProofStateNone,
					Disclosure:          actionDomain.DisclosureDenied,
					ActorID:             servicingWireActor, RecordedAt: time.Date(2026, 10, 25, 12, 10, 0, 0, time.UTC),
				},
				IntentShortWord: actionDomain.ServicingIntentShortWords[actionDomain.ServicingIntentCancel],
				KindShortWord:   actionDomain.LookupKindShortWords[actionDomain.LookupKindSupplierOrderID],
				KindSourceWord:  actionDomain.LookupKindSourceWords[actionDomain.LookupKindSupplierOrderID],
				OutcomeWord:     actionDomain.LookupOutcomeWords[actionDomain.LookupOutcomeUnique],
				Sources:         []string{actionDomain.OrderFactSourceManagerRecord},
				ClientWords:     []string{actionDomain.ServicingGateWord([]string{"перезвонок на известный номер", "ответ на письмо с известного адреса"}), "Повторите проверку разрешённым способом."},
			},
			{
				Request: actionDomain.OrderServicingRequest{
					RequestNo: 3, Intent: actionDomain.ServicingIntentCancel,
					IdentifierKind:      actionDomain.LookupKindABCPClientOrderID,
					IdentifierValue:     "ABCP-ORDER-66",
					ResolvedOperationID: servicingWireOper,
					LookupOutcome:       actionDomain.LookupOutcomeUnique,
					ProofState:          actionDomain.ProofStateVerified,
					ProofMethod:         "known_number_callback",
					Disclosure:          actionDomain.DisclosureAllowed,
					ActorID:             servicingWireActor, RecordedAt: time.Date(2026, 10, 25, 12, 20, 0, 0, time.UTC),
				},
				IntentShortWord: actionDomain.ServicingIntentShortWords[actionDomain.ServicingIntentCancel],
				KindShortWord:   actionDomain.LookupKindShortWords[actionDomain.LookupKindABCPClientOrderID],
				KindSourceWord:  actionDomain.LookupKindSourceWords[actionDomain.LookupKindABCPClientOrderID],
				OutcomeWord:     actionDomain.LookupOutcomeWords[actionDomain.LookupOutcomeUnique],
				Sources:         []string{actionDomain.OrderFactSourceSystemReceipt},
				ClientWords:     []string{actionDomain.ServicingIntentCancelWord},
				Cancel: &actionDomain.CancelFeasibilityView{
					RequestRecordedWord: actionDomain.ServicingCancelRequestRecordedWord,
					PreparedActionWord:  actionDomain.ServicingCancelNoPreparedActionWord,
					ConfirmedWord:       actionDomain.ServicingCancelNoConfirmationWord,
					NotEditableWord:     actionDomain.ServicingCancelSupplierTouchedWord,
					SupplierTouched:     true,
					ManualContourWord:   actionDomain.ServicingCancelManualWord,
				},
			},
		},
	}
}

// TestOrderServicingPanelRendersTheFeed pins the panel rendering: the
// request feed with the proof pins (the state word, the method, the
// two times), the TYPED identifier with the kind words and the source
// pins BY NAME, the lookup outcome word, the disclosure state TEXT
// (never a color only), the LIVE client words, the four cancel states
// and the record form with the intent select of the 8 words and the
// kind select of the 2 words (AC-16).
func TestOrderServicingPanelRendersTheFeed(t *testing.T) {
	surface := &fakeOrderServicingSurface{view: servicingWireView()}
	server := newOrderServicingWireServer(t, surface)
	panel, err := server.orderServicingPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, servicingWireTenant, servicingWireCase)
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if !panel.Wired || len(panel.Requests) != 3 {
		t.Fatalf("the panel is wired with the three requests: %+v", panel)
	}
	first := panel.Requests[0]
	if first.Intent != actionDomain.ServicingIntentOrderStatus || first.Outcome != actionDomain.LookupOutcomeUnique {
		t.Fatalf("the first request: %+v", first)
	}
	if first.ProofState == "" || !strings.Contains(first.ProofState, actionDomain.ProofStateVerified) {
		t.Fatalf("the proof state word: %q", first.ProofState)
	}
	if !strings.Contains(first.Sources, actionDomain.OrderFactSourceSystemReceipt) || !strings.Contains(first.Sources, actionDomain.OrderFactSourceManagerRecord) {
		t.Fatalf("the source pins BY NAME: %q", first.Sources)
	}
	if first.DisclosureWord == "" || !strings.Contains(first.DisclosureWord, "verified") {
		t.Fatalf("the disclosure word: %q", first.DisclosureWord)
	}
	second := panel.Requests[1]
	if !strings.Contains(strings.Join(second.ClientWords, " "), actionDomain.ServicingGateWordPrefix) {
		t.Fatalf("the denied client words carry the gate word: %v", second.ClientWords)
	}
	third := panel.Requests[2]
	if third.Cancel == nil || !strings.Contains(third.Cancel.NotEditable, actionDomain.ServicingCancelSupplierTouchedWord) {
		t.Fatalf("the cancel four states: %+v", third.Cancel)
	}
	// The template render: the section words, the feed table, the four
	// cancel states, the client words, the record form and the stable
	// word-links of the human contours.
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": servicingWireTenant.String(),
		"View": caseDetailPageView{CaseID: servicingWireCase.String(), OrderServicing: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the servicing panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Обслуживание заказа",
		"order-servicing",
		"order_status",
		"ABCP-ORDER-66",
		"SUP-66",
		actionDomain.LookupOutcomeWords[actionDomain.LookupOutcomeUnique],
		actionDomain.OrderFactSourceWords[actionDomain.OrderFactSourceSystemReceipt],
		"личность подтверждена (verified)",
		"раскрытие закрыто до проверки личности",
		actionDomain.ServicingGateWordPrefix,
		"Отмена обращения №3: четыре состояния",
		actionDomain.ServicingCancelNoPreparedActionWord,
		actionDomain.ServicingCancelNoConfirmationWord,
		actionDomain.ServicingCancelSupplierTouchedWord,
		actionDomain.ServicingCancelManualWord,
		"Клиентские слова обращения №1",
		actionDomain.OrderStageSupplierSentWord,
		actionDomain.ServicingPhoneSearchBoundaryWord,
		actionDomain.ServicingConnectorBoundaryWord,
		actionDomain.ServicingNoDeliveryWord,
		"Записать обращение по существующему заказу",
		"статус заказа",
		"передача человеку",
		"номер заказа клиента в ABCP",
		"номер заказа у поставщика",
		"identifier_value",
		"Позвать человека",
		"Вернуться в разговор",
		"#handoff",
		"/order-servicing-requests",
	} {
		if !strings.Contains(rendered, pin) {
			t.Fatalf("the rendered page misses %q", pin)
		}
	}
	// The AC-1 leak probe of the RENDERED page: the DENIED request row
	// renders its client words (the gate word) but the denied words NEVER
	// carry the lookup outcome word or the order stage material.
	deniedWords := strings.Join(panel.Requests[1].ClientWords, " ")
	if strings.Contains(deniedWords, actionDomain.LookupOutcomeWords[actionDomain.LookupOutcomeUnique]) ||
		strings.Contains(deniedWords, actionDomain.OrderStageSupplierSentWord) {
		t.Fatalf("the denied client words leak the order material (AC-1): %s", deniedWords)
	}
	// The eight intent options and the two kind options ride the form.
	if len(panel.IntentOptions) != len(actionDomain.ServicingIntents) {
		t.Fatalf("the intent options: %d", len(panel.IntentOptions))
	}
	if len(panel.KindOptions) != len(actionDomain.LookupIdentifierKinds) {
		t.Fatalf("the kind options: %d", len(panel.KindOptions))
	}
}

// TestOrderServicingPanelDegradation pins the honest degradation: an
// unwired servicing surface renders the zero panel (the page keeps the
// rest); a scope-forbidden member sees the zero panel too.
func TestOrderServicingPanelDegradation(t *testing.T) {
	server, err := New(fakeCorrectionWireIdentity{fakeIdentity: fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		}}}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	panel, err := server.orderServicingPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, servicingWireTenant, servicingWireCase)
	if err != nil {
		t.Fatalf("the unwired panel read fails: %v", err)
	}
	if panel.Wired || len(panel.Requests) != 0 {
		t.Fatalf("the unwired panel is the zero panel: %+v", panel)
	}
	forbidden := &fakeOrderServicingSurface{readErr: actionDomain.ErrForbidden}
	server.orderServicing = forbidden
	panel, err = server.orderServicingPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, servicingWireTenant, servicingWireCase)
	if err != nil {
		t.Fatalf("the forbidden panel read fails: %v", err)
	}
	if panel.Wired || len(panel.Requests) != 0 {
		t.Fatalf("the forbidden panel is the zero panel: %+v", panel)
	}
}

// TestOrderServicingOpenAPIContractMirrors pins the OpenAPI contract
// mirrors of the servicing contour: the record path, the
// request/outcome schemas with the enum mirrors (the 8 intents, the 2
// kinds, the 3 outcomes, the 4 proof states, the 2 disclosures) and
// the FENCE probes (no phone/email lookup kind — OQ-16; no
// supplier-send field word — OQ-3; no notification delivery field —
// OQ-1/OQ-2; no price field — the purchasing data stays hidden).
func TestOrderServicingOpenAPIContractMirrors(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatalf("the OpenAPI contract is unreadable: %v", err)
	}
	text := string(raw)
	for _, pin := range []string{
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-servicing-requests:",
		"OrderServicingRequest",
		"OrderServicingOutcome",
		"enum: [order_status, clarification, change, cancel, return, claim, dispute, human]",
		"enum: [abcp_client_order_id, supplier_order_id]",
		"enum: [unique_match, not_found, ambiguous]",
		"enum: [none, basic, verified, step_up]",
		"enum: [allowed, denied]",
		"replay",
	} {
		if !strings.Contains(text, pin) {
			t.Fatalf("the OpenAPI contract misses %q", pin)
		}
	}
	// The OQ-16 fence: the request schema carries NO phone/email lookup
	// kind PROPERTY (the cross-case profile does not exist; the honest
	// fence word of the description names the absence — the probe reads
	// the property syntax, never the words).
	requestSchema := between(text, "OrderServicingRequest:", "OrderServicingOutcome:")
	for _, forbidden := range []string{"phone:", "msisdn:", "email:", "phone_number", "caller_id:"} {
		if strings.Contains(strings.ToLower(requestSchema), forbidden) {
			t.Fatalf("the request schema carries the forbidden lookup kind %q", forbidden)
		}
	}
	// The supplier-send fence: the request schema carries NO send/cancel
	// execution field (the request records the ask, it never performs it —
	// the OQ-3 fence).
	for _, forbidden := range []string{"send_to_supplier", "supplier_send", "dispatch_supplier", "cancel_order", "execute_cancel"} {
		if strings.Contains(requestSchema, forbidden) {
			t.Fatalf("the request schema carries the forbidden execution field %q", forbidden)
		}
	}
	// The notification fence: the outcome schema carries the honest
	// recorded word, never a delivery field (OQ-1/OQ-2).
	outcomeSchema := between(text, "OrderServicingOutcome:", "OrderStatusView:")
	for _, forbidden := range []string{"delivery_status", "read_at", "notified_at", "sent_at"} {
		if strings.Contains(outcomeSchema, forbidden) {
			t.Fatalf("the outcome schema carries the forbidden notification field %q", forbidden)
		}
	}
	// The purchasing fence: the outcome schema carries NO price field.
	for _, forbidden := range []string{"unit_price", "client_price", "purchasing_price", "known_client_sum"} {
		if strings.Contains(outcomeSchema, forbidden) {
			t.Fatalf("the outcome schema carries the forbidden price field %q", forbidden)
		}
	}
}
