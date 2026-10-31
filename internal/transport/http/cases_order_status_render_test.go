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

	"github.com/staffy-co/staffy/internal/action/api"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.9 status wire shapes: the panel rendering (the five stage
// rows with the evidence pins, the per-system blocks with the sources
// BY NAME, the timeline, the three deadline words, the boundary blocks,
// the client retell words), the record form (the kind select, the
// supplier order ID, the named fact time) and the OpenAPI contract
// mirrors with the fence probes (no supplier-send field — OQ-3; no
// notification delivery field — OQ-1/OQ-2; no price field — the
// purchasing data stays hidden).
// ---------------------------------------------------------------------------

// The fixture identities of the status wire world.
var (
	statusWireTenant = id.MustParse("01994f64-6c00-7000-8000-000000000002")
	statusWireCase   = id.MustParse("01994f64-6c00-7000-8000-00000000aa02")
	statusWireOper   = id.MustParse("01994f64-6c00-7000-8000-00000000ee02")
	statusWireOwner  = id.MustParse("01994f64-6c00-7000-8000-00000000b202")
)

// fakeOrderStatusSurface is the scriptable OrderStatusService of the
// wire tests: the read answers ride the fields, the commands answer
// zero outcomes (the command ladder is pinned by the app tests).
type fakeOrderStatusSurface struct {
	projection actionDomain.OrderStatusProjection
	readErr    error
}

func (f *fakeOrderStatusSurface) RecordOrderSupplierFormed(context.Context, api.RecordOrderSupplierFactCommand) (api.OrderSupplierFactOutcome, error) {
	return api.OrderSupplierFactOutcome{}, nil
}
func (f *fakeOrderStatusSurface) RecordOrderSupplierSent(context.Context, api.RecordOrderSupplierFactCommand) (api.OrderSupplierFactOutcome, error) {
	return api.OrderSupplierFactOutcome{}, nil
}
func (f *fakeOrderStatusSurface) ReadOrderStatusView(context.Context, identityDomain.Principal, id.UUID, id.UUID, time.Time) (actionDomain.OrderStatusProjection, error) {
	return f.projection, f.readErr
}

// newOrderStatusWireServer wires the server of the wire tests over the
// scriptable status surface.
func newOrderStatusWireServer(t *testing.T, surface *fakeOrderStatusSurface) *Server {
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
	server.orderStatus = surface
	return server
}

// statusWireProjection builds the full projection fixture of the wire
// tests: the receipt-confirmed creation, the approved verification, the
// formed+sent facts, the per-system blocks and the timeline.
func statusWireProjection() actionDomain.OrderStatusProjection {
	happened := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	projection := actionDomain.OrderStatusProjection{
		OperationID:  statusWireOper,
		CurrentStage: actionDomain.OrderStageSupplierSent,
		Stages: []actionDomain.OrderStageEvidence{
			{Stage: actionDomain.OrderStageClientOrderCreated, Confirmed: true,
				Word: actionDomain.OrderStageClientCreatedWord, Time: happened.Add(-2 * time.Hour),
				Source: actionDomain.OrderFactSourceSystemReceipt, ExternalID: "ABCP-ORDER-55"},
			{Stage: actionDomain.OrderStagePendingManagerVerification, Confirmed: false,
				Word: "не подтверждено: активной задачи проверки нет: назначьте проверку (контур 4-7)"},
			{Stage: actionDomain.OrderStageManagerVerified, Confirmed: true,
				Word: actionDomain.OrderStageManagerVerifiedWord, Time: happened.Add(-time.Hour),
				Source: actionDomain.OrderFactSourceManagerRecord, ExternalID: "ABCP-ORDER-55"},
			{Stage: actionDomain.OrderStageSupplierFormed, Confirmed: true,
				Word: actionDomain.OrderStageSupplierFormedWord, Time: happened,
				Source: actionDomain.OrderFactSourceManagerRecord, ExternalID: "SUP-55"},
			{Stage: actionDomain.OrderStageSupplierSent, Confirmed: true,
				Word: actionDomain.OrderStageSupplierSentWord, Time: happened.Add(10 * time.Minute),
				Source: actionDomain.OrderFactSourceManagerRecord, ExternalID: "SUP-55"},
		},
		Systems: []actionDomain.OrderStatusSystemView{
			{System: actionDomain.OrderStatusSystemABCP, Word: "создан (квитанция диспетчерского контура)",
				Facts: []actionDomain.OrderStatusFactView{{
					System: actionDomain.OrderStatusSystemABCP, Kind: "receipt_committed",
					ExternalID: "ABCP-ORDER-55", RecordedAt: happened.Add(-2 * time.Hour),
					Source: actionDomain.OrderFactSourceSystemReceipt,
					Word:    "внешний ID ABCP Order из квитанции операции: ABCP-ORDER-55"}}},
			{System: actionDomain.OrderStatusSystemOneC, Word: actionDomain.OrderOneCUnknownWord},
			{System: actionDomain.OrderStatusSystemSupplier, Word: actionDomain.OrderStatusSupplierManagerSourceWord,
				Facts: []actionDomain.OrderStatusFactView{
					{System: actionDomain.OrderStatusSystemSupplier, Kind: actionDomain.OrderSupplierFactFormed,
						ExternalID: "SUP-55", HappenedAt: happened, RecordedAt: happened.Add(time.Minute),
						Source: actionDomain.OrderFactSourceManagerRecord, ActorID: statusWireOwner,
						Word:   "сформирован у поставщика (запись менеджера) — SUP-55"},
					{System: actionDomain.OrderStatusSystemSupplier, Kind: actionDomain.OrderSupplierFactSent,
						ExternalID: "SUP-55", HappenedAt: happened.Add(10 * time.Minute), RecordedAt: happened.Add(11 * time.Minute),
						Source: actionDomain.OrderFactSourceManagerRecord, ActorID: statusWireOwner,
						Word:   "отправлен поставщику (запись менеджера) — SUP-55"},
				}},
		},
		Timeline: []actionDomain.OrderStatusTimelineEntry{
			{At: happened.Add(-2 * time.Hour), Machine: "order", Kind: "receipt_committed",
				Word: actionDomain.OrderStageClientCreatedWord, Source: actionDomain.OrderFactSourceSystemReceipt,
				ExternalID: "ABCP-ORDER-55"},
			{At: happened, Machine: "supplier_fact", Kind: "supplier_fact_formed",
				Word: "сформирован у поставщика (запись менеджера) — SUP-55",
				Source: actionDomain.OrderFactSourceManagerRecord, ExternalID: "SUP-55"},
		},
		Freshness: actionDomain.OrderStatusFreshness{
			LastConfirmedAt: happened.Add(11 * time.Minute),
			Word:            actionDomain.OrderStatusFreshnessWord,
			NoScheduleWord:  actionDomain.OrderStatusNoScheduleWord,
		},
		Deadline: actionDomain.OrderStatusDeadlineBlock{
			EstimateWord:   actionDomain.OrderStatusDeadlineEstimateWord + " — срок 2-3 дня",
			ConfirmedWord:  actionDomain.OrderStatusDeadlineConfirmedWord,
			ActualWord:     actionDomain.OrderStatusDeadlineActualWord + " — 2026-10-25T10:10:00Z",
			ActualAt:       happened.Add(10 * time.Minute),
			ActualRecorded: true,
		},
		NotificationWord: actionDomain.OrderStatusNoNotificationsWord,
		DisputeWord:      actionDomain.OrderStatusDisputeBoundaryWord,
		CancellationWord: actionDomain.OrderStatusCorrectionBoundaryWord,
		NextStep:         "ожидание поставки: записей о поставке в этом срезе нет — стадия «отправлен» подтверждена записями магазина",
		ClientWords:      []string{actionDomain.OrderStageSupplierSentWord, actionDomain.OrderClientBoundaryWord, actionDomain.OrderOneCUnknownWord},
	}
	return projection
}

// TestOrderStatusPanelRendersTheProjection pins the panel rendering:
// the five stage rows with the evidence (the source BY NAME with its
// text, the external ID, the time), the per-system blocks (ABCP /
// 1С unknown / поставщик with the manager facts), the timeline, the
// three deadline words, the boundary blocks and the record form with
// the kind select (AC-16: the stages are WORDS, never colors only).
func TestOrderStatusPanelRendersTheProjection(t *testing.T) {
	surface := &fakeOrderStatusSurface{projection: statusWireProjection()}
	server := newOrderStatusWireServer(t, surface)
	panel, err := server.orderStatusPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, statusWireTenant, statusWireCase)
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if !panel.Wired || !panel.HasOrder || len(panel.Stages) != 5 {
		t.Fatalf("the panel is wired with the five stages: %+v", panel)
	}
	if panel.Current != actionDomain.OrderStageSupplierSent {
		t.Fatalf("the current stage: %q", panel.Current)
	}
	if panel.OperationID != statusWireOper.String() {
		t.Fatalf("the anchor operation: %q", panel.OperationID)
	}
	created := panel.Stages[0]
	if !created.Confirmed || created.Source != actionDomain.OrderFactSourceSystemReceipt {
		t.Fatalf("the created stage evidence: %+v", created)
	}
	if created.SourceText != actionDomain.OrderFactSourceWords[actionDomain.OrderFactSourceSystemReceipt] {
		t.Fatalf("the source text: %q", created.SourceText)
	}
	if created.ExternalID != "ABCP-ORDER-55" {
		t.Fatalf("the external ID: %q", created.ExternalID)
	}
	if len(panel.Systems) != 3 {
		t.Fatalf("the per-system blocks: %d", len(panel.Systems))
	}
	if !strings.Contains(panel.Systems[1].Word, "unknown") {
		t.Fatalf("the 1C word: %q", panel.Systems[1].Word)
	}
	if len(panel.Systems[2].Facts) != 2 {
		t.Fatalf("the supplier facts: %d", len(panel.Systems[2].Facts))
	}
	for _, fact := range panel.Systems[2].Facts {
		if fact.Source != actionDomain.OrderFactSourceManagerRecord {
			t.Fatalf("the supplier fact source: %q", fact.Source)
		}
	}
	if len(panel.Timeline) != 2 {
		t.Fatalf("the timeline rows: %d", len(panel.Timeline))
	}
	if !panel.DeadlineEstContains() {
		t.Fatalf("the estimate word: %q", panel.DeadlineEst)
	}
	// The template render: the section words, the stage table, the
	// per-system blocks, the timeline, the three deadline words, the
	// boundary blocks, the record form and the client retell words.
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": statusWireTenant.String(),
		"View": caseDetailPageView{CaseID: statusWireCase.String(), OrderStatus: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the status panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Статус заказа",
		"order-status",
		"client_order_created",
		"supplier_sent",
		actionDomain.OrderStageSupplierSentWord,
		"подтверждено",
		"не подтверждено",
		"ABCP (магазин)",
		"1С (бухгалтерия)",
		actionDomain.OrderOneCUnknownWord,
		"По системам раздельно",
		"ABCP-ORDER-55",
		"SUP-55",
		actionDomain.OrderFactSourceWords[actionDomain.OrderFactSourceManagerRecord],
		"Лента переходов",
		"receipt_committed",
		"supplier_fact_formed",
		"Сроки",
		actionDomain.OrderStatusDeadlineConfirmedWord,
		"Свежесть",
		actionDomain.OrderStatusFreshnessWord,
		actionDomain.OrderStatusNoScheduleWord,
		actionDomain.OrderStatusNoNotificationsWord,
		actionDomain.OrderStatusDisputeBoundaryWord,
		"Записать факт поставщика",
		"Сформирован у поставщика",
		"Отправлен поставщику",
		"supplier_order_id",
		"happened_at",
		"Клиентские слова",
		actionDomain.OrderClientBoundaryWord,
		statusWireOper.String(),
	} {
		if !strings.Contains(rendered, pin) {
			t.Fatalf("the rendered page misses %q", pin)
		}
	}
	// The record form addresses the PRG route with the anchor operation.
	if !strings.Contains(rendered, "/order-supplier-facts") {
		t.Fatal("the record form rides the PRG route")
	}
}

// DeadlineEstContains reports the honest estimate material of the
// deadline block (the helper reads the fixture word).
func (v orderStatusPanelView) DeadlineEstContains() bool {
	return strings.Contains(v.DeadlineEst, "срок 2-3 дня")
}

// TestOrderStatusPanelSpecialState pins the special state rendering
// (AC-12): the open correction word rides the panel — never hidden
// behind «в работе».
func TestOrderStatusPanelSpecialState(t *testing.T) {
	projection := statusWireProjection()
	projection.SpecialState = actionDomain.OrderCorrectionPendingWord
	projection.SpecialOwner = statusWireOwner
	surface := &fakeOrderStatusSurface{projection: projection}
	server := newOrderStatusWireServer(t, surface)
	panel, err := server.orderStatusPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, statusWireTenant, statusWireCase)
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if panel.Special != actionDomain.OrderCorrectionPendingWord {
		t.Fatalf("the special state: %q", panel.Special)
	}
	if panel.SpecialOwn != statusWireOwner.String() {
		t.Fatalf("the special owner: %q", panel.SpecialOwn)
	}
}

// TestOrderStatusPanelDegradation pins the honest degradation: an
// unwired status surface renders the zero panel (the page keeps the
// rest); a scope-forbidden member sees the zero panel too.
func TestOrderStatusPanelDegradation(t *testing.T) {
	server, err := New(fakeCorrectionWireIdentity{fakeIdentity: fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		}}}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	panel, err := server.orderStatusPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, statusWireTenant, statusWireCase)
	if err != nil {
		t.Fatalf("the unwired panel read fails: %v", err)
	}
	if panel.Wired || len(panel.Stages) != 0 {
		t.Fatalf("the unwired panel is the zero panel: %+v", panel)
	}
	// The scope-forbidden read: the zero panel, the page keeps the rest.
	forbidden := &fakeOrderStatusSurface{readErr: actionDomain.ErrForbidden}
	server.orderStatus = forbidden
	panel, err = server.orderStatusPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, statusWireTenant, statusWireCase)
	if err != nil {
		t.Fatalf("the forbidden panel read fails: %v", err)
	}
	if panel.Wired || len(panel.Stages) != 0 {
		t.Fatalf("the forbidden panel is the zero panel: %+v", panel)
	}
}

// TestOrderStatusOpenAPIContractMirrors pins the OpenAPI contract
// mirrors of the status contour: the two record paths, the
// request/outcome schemas, the OrderStatusView closed shape, the enum
// mirrors of the dictionaries and the FENCE probes (no supplier-send
// field word, no notification delivery field, no price field —
// OQ-3/OQ-1/OQ-2).
func TestOrderStatusOpenAPIContractMirrors(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatalf("the OpenAPI contract is unreadable: %v", err)
	}
	text := string(raw)
	for _, pin := range []string{
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-supplier-facts/formed:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-supplier-facts/sent:",
		"OrderSupplierFactRequest",
		"OrderSupplierFactOutcome",
		"OrderStatusView",
		"enum: [formed, sent]",
		"enum: [system_receipt, manager_record]",
		"enum: [client_order_created, pending_manager_verification, manager_verified, supplier_formed, supplier_sent]",
		"enum: [abcp, one_c, supplier]",
		"enum: [order, verification, correction, supplier_fact]",
		"replay",
	} {
		if !strings.Contains(text, pin) {
			t.Fatalf("the OpenAPI contract misses %q", pin)
		}
	}
	// The supplier-send fence of the contract: the request schema of the
	// facts carries NO send field (the record IS the claim, never the
	// command — the OQ-3 structural probe).
	factSchema := between(text, "OrderSupplierFactRequest:", "OrderSupplierFactOutcome:")
	for _, forbidden := range []string{"send_to_supplier", "supplier_send", "dispatch_supplier"} {
		if strings.Contains(factSchema, forbidden) {
			t.Fatalf("the fact request schema carries the forbidden supplier-send field %q", forbidden)
		}
	}
	// The notification fence: the OrderStatusView carries the honest
	// notification WORD, never a delivery field (OQ-1/OQ-2).
	statusSchema := between(text, "OrderStatusView:", "NewPreviewVersionRequest:")
	for _, forbidden := range []string{"delivery_status", "read_at", "notified_at"} {
		if strings.Contains(statusSchema, forbidden) {
			t.Fatalf("the status view schema carries the forbidden notification field %q", forbidden)
		}
	}
	// The purchasing fence: the status view carries NO price field (the
	// purchasing data stays hidden — the Commercial scope of the other
	// surfaces).
	for _, forbidden := range []string{"unit_price", "client_price", "purchasing_price", "known_client_sum"} {
		if strings.Contains(statusSchema, forbidden) {
			t.Fatalf("the status view schema carries the forbidden price field %q", forbidden)
		}
	}
}

// between extracts the section between the two markers (the honest
// helper of the contract probes).
func between(text, start, end string) string {
	startIdx := strings.Index(text, start)
	if startIdx < 0 {
		return ""
	}
	rest := text[startIdx:]
	endIdx := strings.Index(rest, end)
	if endIdx < 0 {
		return rest
	}
	return rest[:endIdx]
}
