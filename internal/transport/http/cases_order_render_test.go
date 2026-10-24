package httptransport

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.6 order wire shapes: the confirmation card rendering (the
// material lines, the MANDATORY manager-check word AC-1, the two
// DIFFERENT buttons of the two DIFFERENT actions), the receipt words
// (the derived state, the client words, the honest 1C-unknown inside
// role=alert), the AG-4 recommendation of the parked operation, the
// honest no-selection word and the OpenAPI contract mirrors.
// ---------------------------------------------------------------------------

// orderWireSurface builds the action answers of the wire tests: the
// CURRENT preview card of the order definition with the typed input of a
// built material, and the order operation receipts with the derived
// words of the committed outcome.
func orderWireSurface() (api.PreviewView, api.AuthorizationView) {
	caseID := id.MustParse("01994f64-6c00-7000-8000-00000000aa01")
	workItemID := id.MustParse("01994f64-6c00-7000-8000-00000000bb01")
	previewID := id.MustParse("01994f64-6c00-7000-8000-00000000cc01")
	material := "позиция WF2B196K666AA: Mann / WF2B196K666AA (оригинал); кол-во 2; цена 4200.00 RUB\n" +
		"итого: 8400.00 RUB (без доставки и сбора — окончательная стоимость не рассчитана)\n" +
		actionDomain.OrderManagerCheckWord
	card := api.PreviewCardView{
		PreviewID:         previewID,
		VersionNo:         1,
		DefinitionID:      actionDomain.OrderActionID,
		DefinitionVersion: actionDomain.OrderActionVersion,
		CaseID:            caseID,
		WorkItemID:        workItemID,
		TypedInput:        map[string]string{"order_digest": "d1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", "order_material": material, "known_client_sum": "8400.00", "currency": "RUB"},
		TimelineText:      "предложение построено",
		ReadinessText:     "материал готов",
		Current:           true,
	}
	previews := api.PreviewView{TenantID: id.MustParse("01994f64-6c00-7000-8000-000000000002"), CaseID: caseID,
		Threads: []api.PreviewThreadView{{PreviewID: previewID, Cards: []api.PreviewCardView{card}}}}
	receipt := api.ActionReceiptView{
		ReceiptVersionNo:      1,
		Outcome:               actionDomain.ReceiptOutcomeCommitted,
		OutcomeText:           actionDomain.OperationCommittedText,
		CaseID:                caseID,
		WorkItemID:            workItemID,
		PreviewID:             previewID,
		PreviewVersionNo:      1,
		ConfirmationID:        id.MustParse("01994f64-6c00-7000-8000-00000000dd01"),
		ConfirmationVersionNo: 1,
		DefinitionID:          actionDomain.OrderActionID,
		DefinitionVersion:     actionDomain.OrderActionVersion,
		ExternalObjectID:      "ABCP-ORDER-42",
		ExternalObjectVersion: "3",
		OrderState:            actionDomain.OrderReceiptStatePendingManagerVerification,
		OrderStateWord:        actionDomain.OrderClientCreatedWord,
		OrderBoundaryWord:     actionDomain.OrderClientBoundaryWord,
		OrderOneCWord:         actionDomain.OrderOneCUnknownWord,
		OrderManagerWord:      actionDomain.OrderManagerAssignmentWord,
	}
	operations := api.AuthorizationView{TenantID: previews.TenantID, CaseID: caseID,
		Operations: []api.OperationCardView{{
			OperationID:  id.MustParse("01994f64-6c00-7000-8000-00000000ee01"),
			DefinitionID: actionDomain.OrderActionID, DefinitionVersion: actionDomain.OrderActionVersion,
			PreviewID: previewID, PreviewVersionNo: 1,
			CaseID: caseID, WorkItemID: workItemID,
			Receipts: []api.ActionReceiptView{receipt},
		}}}
	return previews, operations
}

// orderWireCaseView builds the case view with ONE recorded selection (the
// buildable panel input).
func orderWireCaseView() casesAPI.CaseView {
	return casesAPI.CaseView{Comparison: casesAPI.ComparisonPanelView{
		SchemaDefined: true,
		Selections: []casesDomain.SelectionSnapshotView{{
			SnapshotNo: 1, PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 1,
			Quantity: 2, UnitClientPrice: 4200, Currency: "RUB", KnownClientSum: 8400,
			ProfileDigestPin: strings.Repeat("a", 64), PartRequestDigestPin: strings.Repeat("b", 64),
			FitmentDigestPin: strings.Repeat("c", 64), OfferDigestPin: strings.Repeat("d", 64),
			SelectionDigest: strings.Repeat("e", 64),
		}},
	}}
}

// TestOrderPanelRendersTheConfirmationCard pins the confirmation card
// rendering: the material lines, the known sum with the boundary word,
// the MANDATORY manager-check word (AC-1), the digest pin and the TWO
// DIFFERENT buttons of the two DIFFERENT actions (the confirmation
// through the existing 3-4 route; the correction through the builder
// re-derivation — AC-16).
func TestOrderPanelRendersTheConfirmationCard(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	previews, operations := orderWireSurface()
	panel := prepareOrderPanel(orderWireCaseView(), previews, operations,
		id.MustParse("01994f64-6c00-7000-8000-000000000002"), id.MustParse("01994f64-6c00-7000-8000-00000000aa01"))
	if !panel.PreviewPresent || !panel.Buildable {
		t.Fatal("the built order panel is present and buildable")
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002",
		"View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Order: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the order panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Заказ клиента",
		"Карточка подтверждения заказа",
		actionDomain.OrderActionID + "@" + actionDomain.OrderActionVersion,
		"позиция WF2B196K666AA: Mann / WF2B196K666AA (оригинал); кол-во 2; цена 4200.00 RUB",
		"итого: 8400.00 RUB",
		actionDomain.OrderSumBoundaryWord,
		actionDomain.OrderManagerCheckWord,
		"d1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"Подтвердить точную версию заказа",
		"Пересобрать заказ из живого выбора",
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the order panel must carry %q", pin)
		}
	}
	// AC-16: the confirmation and the correction are DIFFERENT actions —
	// two DIFFERENT form actions (the existing 3-4 confirmations route;
	// the builder order-proposals route).
	confirmRoute := "/operations/confirmations"
	rebuildRoute := "/order-proposals"
	if !strings.Contains(rendered, confirmRoute) {
		t.Fatalf("the confirm button must post to the existing confirmations route %q", confirmRoute)
	}
	if !strings.Contains(rendered, rebuildRoute) {
		t.Fatalf("the correction button must post to the builder route %q", rebuildRoute)
	}
	// The creation is never masked as a supplier send (AC-16 tail): no
	// supplier-send word rides the panel.
	for _, forbidden := range []string{"отправить поставщику", "supplier_form", "supplier_send"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("the order panel never speaks the supplier-send word %q", forbidden)
		}
	}
}

// TestOrderPanelRendersTheReceiptWords pins the receipt rendering: the
// derived state word with the client words, the honest 1C-unknown word
// INSIDE role=alert and the manager assignment word.
func TestOrderPanelRendersTheReceiptWords(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	previews, operations := orderWireSurface()
	panel := prepareOrderPanel(orderWireCaseView(), previews, operations,
		id.MustParse("01994f64-6c00-7000-8000-000000000002"), id.MustParse("01994f64-6c00-7000-8000-00000000aa01"))
	if !panel.HasReceipts || len(panel.Receipts) != 1 {
		t.Fatalf("one order receipt row, got %d", len(panel.Receipts))
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002",
		"View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Order: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the order receipts: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Квитанции заказа",
		"Квитанция №1",
		"ABCP-ORDER-42",
		actionDomain.OrderReceiptStatePendingManagerVerification,
		actionDomain.OrderClientCreatedWord,
		actionDomain.OrderClientBoundaryWord,
		actionDomain.OrderOneCUnknownWord,
		actionDomain.OrderManagerAssignmentWord,
	} {
		if !strings.Contains(rendered, pin) {
			t.Errorf("the order receipt must carry %q", pin)
		}
	}
	// The 1C-unknown word renders as a status difference — role=alert,
	// never only color (AC-14/the a11y discipline).
	if !strings.Contains(rendered, "role=\"alert\"") {
		t.Fatal("the 1C-unknown word must ride inside role=alert")
	}
}

// TestOrderPanelRendersTheAG4Recommendation pins the parked-operation
// recommendation: the honest AG-4 word inside role=alert and the
// order_creation handoff reason word with its honest text — the panel
// recommends, the operator decides (never an automatic send).
func TestOrderPanelRendersTheAG4Recommendation(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	previews, operations := orderWireSurface()
	// The parked contour: the order operation carries NO committed
	// receipt (the production nil-adapter state).
	operations.Operations[0].Receipts = nil
	panel := prepareOrderPanel(orderWireCaseView(), previews, operations,
		id.MustParse("01994f64-6c00-7000-8000-000000000002"), id.MustParse("01994f64-6c00-7000-8000-00000000aa01"))
	if !panel.AG4Recommended {
		t.Fatal("the order operation without a committed receipt carries the AG-4 recommendation")
	}
	if panel.AG4Word != casesDomain.HandoffReasonOrderCreation {
		t.Fatalf("the recommendation word is order_creation, got %q", panel.AG4Word)
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002",
		"View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Order: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the AG-4 recommendation: %v", err)
	}
	rendered := page.String()
	if !strings.Contains(rendered, casesDomain.HandoffReasonOrderCreation) {
		t.Fatal("the recommendation must carry the order_creation word")
	}
	if !strings.Contains(rendered, casesDomain.HandoffReasonText(casesDomain.HandoffReasonOrderCreation)) {
		t.Fatal("the recommendation must carry the honest reason text")
	}
	if !strings.Contains(rendered, "role=\"alert\"") {
		t.Fatal("the AG-4 block must ride inside role=alert")
	}
}

// TestOrderPanelNoSelectionWord pins the honest degradation: the case
// without recorded selections renders the builder refusal word, never a
// build affordance.
func TestOrderPanelNoSelectionWord(t *testing.T) {
	templates := decisionContextRenderTemplates(t)
	previews, operations := orderWireSurface()
	emptyView := orderWireCaseView()
	emptyView.Comparison.Selections = nil
	panel := prepareOrderPanel(emptyView, previews, operations,
		id.MustParse("01994f64-6c00-7000-8000-000000000002"), id.MustParse("01994f64-6c00-7000-8000-00000000aa01"))
	if panel.Buildable || panel.PreviewPresent {
		t.Fatal("no selections — no build affordance and no preview card")
	}
	if panel.NoSelectionWord != actionDomain.OrderNoSelectionWord {
		t.Fatalf("the honest no-selection word, got %q", panel.NoSelectionWord)
	}
	var page bytes.Buffer
	data := map[string]any{"TenantID": "01994f64-6c00-7000-8000-000000000002",
		"View": caseDetailPageView{CaseID: "01994f64-6c00-7000-8000-00000000aa01", Order: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render the no-selection word: %v", err)
	}
	if !strings.Contains(page.String(), actionDomain.OrderNoSelectionWord) {
		t.Fatal("the page must speak the honest no-selection word")
	}
}

// TestOrderOpenAPIContractMirrorsDictionaries pins the contract mirrors:
// the order proposals path, the empty request body, the closed-shape
// typed input of the definition (NO supplier-send field), the receipt
// state enum, the reason enum word and the schema family names.
func TestOrderOpenAPIContractMirrorsDictionaries(t *testing.T) {
	contract, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	for _, pin := range []string{
		"prepareOrderProposal",
		"order-proposals",
		"OrderProposalRequest",
		"OrderCommitmentTypedInput",
		"create_abcp_customer_order@1.0.0",
		"abcp-parts@1.1.0/command.prepare-client-order",
		"provider_idempotent",
		"required: [order_digest, order_material, known_client_sum, currency]",
		"enum: [pending_manager_verification]",
		"order_creation",
		"cases/00025",
		"fence OQ-3",
	} {
		if !strings.Contains(text, pin) {
			t.Errorf("the contract must mirror %q", pin)
		}
	}
	// The typed input schema block carries NO supplier-send field (the
	// OQ-3 fence — a structural probe over the schema block).
	start := strings.Index(text, "OrderCommitmentTypedInput:")
	if start < 0 {
		t.Fatal("the OrderCommitmentTypedInput schema must exist")
	}
	block := text[start : start+2400]
	// The honest fence DESCRIPTION names the absent words (OQ-3); the FIELDS
	// must not exist: the probe checks the property declarations, not the
	// description prose.
	for _, forbidden := range []string{"supplier_form", "supplier_send", "send_to_supplier", "delivery_to_supplier"} {
		if strings.Contains(block, forbidden+": {") {
			t.Errorf("the order typed input schema must NOT carry the supplier-send field %q", forbidden)
		}
	}
}
