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
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// The Story 4.8 correction wire shapes: the correction card rendering
// (the state word, the round material, the carried blocked material, the
// closure material), the THREE DIFFERENT forms (AC-16: «Записать
// перечит», «Прекратить коррекцию», «Переназначить»), the recommended
// explanation (the explicit former → current differences по позиции, the
// new-confirmation word, the «прежде не отправлялся» boundary, the honest
// channel word) and the OpenAPI contract mirrors with the fence probes
// (no supplier-send field, no change-action material — OQ-3/AG-4).
// ---------------------------------------------------------------------------

// The fixture identities of the correction wire world.
var (
	correctionWireTenant  = id.MustParse("01994f64-6c00-7000-8000-000000000002")
	correctionWireCase    = id.MustParse("01994f64-6c00-7000-8000-00000000aa01")
	correctionWireWork    = id.MustParse("01994f64-6c00-7000-8000-00000000bb01")
	correctionWireOper    = id.MustParse("01994f64-6c00-7000-8000-00000000ee01")
	correctionWireOwner   = id.MustParse("01994f64-6c00-7000-8000-00000000b201")
	correctionWireDigest  = strings.Repeat("f1", 32)
	correctionWireConfirm = id.MustParse("01994f64-6c00-7000-8000-00000000d301")
)

// fakeCorrectionSurface is the scriptable OrderCorrectionService of the
// wire tests: the read answers ride the fields, the commands answer zero
// outcomes (the command ladder is pinned by the app tests).
type fakeCorrectionSurface struct {
	views []actionDomain.OrderCorrectionStateView
}

func (f *fakeCorrectionSurface) OpenOrderCorrection(context.Context, api.OpenOrderCorrectionCommand) (api.OrderCorrectionOutcome, error) {
	return api.OrderCorrectionOutcome{}, nil
}
func (f *fakeCorrectionSurface) ReassignOrderCorrection(context.Context, api.ReassignOrderCorrectionCommand) (api.OrderCorrectionOutcome, error) {
	return api.OrderCorrectionOutcome{}, nil
}
func (f *fakeCorrectionSurface) RecordOrderCorrectionReread(context.Context, api.RecordOrderCorrectionRereadCommand) (api.OrderCorrectionOutcome, error) {
	return api.OrderCorrectionOutcome{}, nil
}
func (f *fakeCorrectionSurface) CeaseOrderCorrection(context.Context, api.CeaseOrderCorrectionCommand) (api.OrderCorrectionOutcome, error) {
	return api.OrderCorrectionOutcome{}, nil
}
func (f *fakeCorrectionSurface) ReadOrderCorrectionCaseView(context.Context, identityDomain.Principal, id.UUID, id.UUID, time.Time) ([]actionDomain.OrderCorrectionStateView, error) {
	return f.views, nil
}

// newCorrectionWireServer wires the server of the wire tests over the
// scriptable surfaces (the verification surface serves the live material
// read of the recommended explanation).
func newCorrectionWireServer(t *testing.T, surface *fakeCorrectionSurface, views []actionDomain.OrderVerificationStateView, material actionDomain.OrderCommitmentMaterial, previews api.PreviewView, authorization api.AuthorizationView) *Server {
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
	server.orderCorrections = surface
	server.orderVerifications = &fakeCorrectionVerificationSurface{views: views, material: material}
	server.actionPreviews = fakeVerificationPreviews{view: previews}
	server.authorization = fakeVerificationAuthorization{view: authorization}
	return server
}

// fakeCorrectionWireIdentity is the permissive identity of the wire
// world (the Commercial gate rides the verification surface identity of
// the 4-7 wire tests; the correction panel reads no purchasing price of
// its own).
type fakeCorrectionWireIdentity struct {
	fakeIdentity
}

// fakeCorrectionVerificationSurface serves the live material read of the
// correction panel (the verification surface read reuse of the spec).
type fakeCorrectionVerificationSurface struct {
	views    []actionDomain.OrderVerificationStateView
	material actionDomain.OrderCommitmentMaterial
}

func (f *fakeCorrectionVerificationSurface) AssignOrderVerification(context.Context, api.AssignOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeCorrectionVerificationSurface) AcknowledgeOrderVerification(context.Context, api.AcknowledgeOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeCorrectionVerificationSurface) ApproveOrderVerification(context.Context, api.ApproveOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeCorrectionVerificationSurface) BlockOrderVerification(context.Context, api.BlockOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeCorrectionVerificationSurface) ReassignOrderVerification(context.Context, api.ReassignOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeCorrectionVerificationSurface) ReadOrderVerificationCaseView(context.Context, identityDomain.Principal, id.UUID, id.UUID, time.Time) ([]actionDomain.OrderVerificationStateView, error) {
	return f.views, nil
}
func (f *fakeCorrectionVerificationSurface) ReadOrderVerificationLiveMaterial(context.Context, identityDomain.Principal, id.UUID, id.UUID) (actionDomain.OrderCommitmentMaterial, error) {
	return f.material, nil
}

// correctionWireStates builds the OPEN correction state view with the
// carried blocked material (the positions and the reason word of the
// opened event — the SQL-truth carry).
func correctionWireStates() []actionDomain.OrderCorrectionStateView {
	correction := actionDomain.OrderCorrection{
		TenantID: correctionWireTenant, CaseID: correctionWireCase, WorkItemID: correctionWireWork,
		CorrectionNo: 1, OperationID: correctionWireOper,
		VerificationNo: 1, ReceiptVersionNo: 1, ExternalOrderID: "ABCP-ORDER-42",
		CreatorID: testPrincipalID, CreatedAt: time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
	}
	opened := actionDomain.OrderCorrectionEvent{
		TenantID: correctionWireTenant, CaseID: correctionWireCase, CorrectionNo: 1, EventNo: 1,
		Kind: actionDomain.OrderCorrectionEventOpened, Owner: correctionWireOwner,
		CorrectDeadline: actionDomain.Correct4h, DeadlineAt: time.Date(2026, 10, 25, 13, 0, 0, 0, time.UTC),
		Positions: []string{"WF2B196K666AA"}, Reason: actionDomain.VerifyBlockReasonWrongPart,
		ActorID: testPrincipalID, CreatedAt: time.Date(2026, 10, 25, 9, 5, 0, 0, time.UTC),
	}
	return []actionDomain.OrderCorrectionStateView{{
		Correction: correction, LastEvent: &opened,
		State: actionDomain.OrderCorrectionStatePending,
		Owner: correctionWireOwner, CorrectDeadline: actionDomain.Correct4h,
		DeadlineAt: opened.DeadlineAt, AssignmentEpoch: 1,
	}}
}

// correctionWireMaterial builds the live material of one position (the
// CURRENT composition of the recommended explanation).
func correctionWireMaterial() actionDomain.OrderCommitmentMaterial {
	return actionDomain.OrderCommitmentMaterial{
		ProfileDigestPin:     strings.Repeat("a", 64),
		PartRequestDigestPin: strings.Repeat("b", 64),
		Positions: []actionDomain.OrderPosition{{
			PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 3,
			Brand: "Mann", Article: "WF2B196K666AA", Originality: actionDomain.OrderOriginalityOriginal,
			Quantity: 2, UnitClientPrice: 4200, Currency: "RUB", Conditions: "2 дня",
			FitmentDigestPin: strings.Repeat("c", 64), OfferDigestPin: strings.Repeat("d", 64),
			SelectionDigest: strings.Repeat("e", 64),
		}},
		KnownClientSum: 8400, Currency: "RUB",
	}
}

// correctionWirePreviews builds the preview view carrying the order
// card of the FORMER version (the pinned render — DIVERGES from the live
// material: the article and the price moved, the diff row renders).
func correctionWirePreviews() api.PreviewView {
	pinned := correctionWireMaterial()
	pinned.Positions[0].Article = "WF2B196K666AB"
	pinned.Positions[0].UnitClientPrice = 4300
	pinned.KnownClientSum = 8600
	material := actionDomain.OrderMaterialCanonicalText(pinned)
	card := api.PreviewCardView{
		PreviewID:         id.MustParse("01994f64-6c00-7000-8000-00000000cc01"),
		VersionNo:         1,
		DefinitionID:      actionDomain.OrderActionID,
		DefinitionVersion: actionDomain.OrderActionVersion,
		CaseID:            correctionWireCase,
		WorkItemID:        correctionWireWork,
		TypedInput:        map[string]string{"order_digest": correctionWireDigest, "order_material": material, "known_client_sum": "8600.00", "currency": "RUB"},
		Current:           true,
	}
	return api.PreviewView{TenantID: correctionWireTenant, CaseID: correctionWireCase,
		Threads: []api.PreviewThreadView{{PreviewID: card.PreviewID, Cards: []api.PreviewCardView{card}}}}
}

// correctionWireAuthorization builds the operation card with the
// committed receipt (the diff anchor of the operation's render).
func correctionWireAuthorization() api.AuthorizationView {
	receipt := api.ActionReceiptView{
		ReceiptVersionNo: 1, Outcome: actionDomain.ReceiptOutcomeCommitted,
		ExternalObjectID: "ABCP-ORDER-42", ExternalObjectVersion: "3",
	}
	return api.AuthorizationView{TenantID: correctionWireTenant, CaseID: correctionWireCase,
		Operations: []api.OperationCardView{{
			OperationID: correctionWireOper, DefinitionID: actionDomain.OrderActionID,
			PreviewID: id.MustParse("01994f64-6c00-7000-8000-00000000cc01"), PreviewVersionNo: 1,
			CaseID: correctionWireCase, WorkItemID: correctionWireWork,
			IdempotencyKey: "order-key-0001", Receipts: []api.ActionReceiptView{receipt},
		}}}
}

// correctionWireCaseView builds the empty case view (the panel reads the
// purchasing price through the verification surface rows — the wire test
// pins the correction panel's own material only).
func correctionWireCaseView() casesAPI.CaseView {
	return casesAPI.CaseView{Comparison: casesAPI.ComparisonPanelView{
		SchemaDefined: true,
		Records: []casesDomain.OfferView{{
			OfferNo: 3, PositionKey: "WF2B196K666AA", SupplierID: "SUP-2",
			Brand: "Mann", Article: "WF2B196K666AA",
			ClientPrice: 4200, Currency: "RUB",
			OfferDigest: strings.Repeat("d", 64),
		}},
	}}
}

// TestOrderCorrectionPanelRendersTheCard pins the correction card
// rendering: the state word with its honest text, the round material
// (the owner by id, the deadline word with its time), the assignment
// epoch, the technical line (the operation, the blocked verification
// number, the original external ABCP Order ID), the carried blocked
// material (the positions with the reason word BY NAME) and the honest
// boundary words of the section (the manual contour, the two structural
// gates).
func TestOrderCorrectionPanelRendersTheCard(t *testing.T) {
	surface := &fakeCorrectionSurface{views: correctionWireStates()}
	server := newCorrectionWireServer(t, surface, nil, correctionWireMaterial(), correctionWirePreviews(), correctionWireAuthorization())
	panel, err := server.orderCorrectionPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, correctionWireTenant, correctionWireCase, correctionWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if !panel.Wired || !panel.Present || len(panel.Cards) != 1 {
		t.Fatalf("the correction card is present: %+v", panel)
	}
	card := panel.Cards[0]
	if card.State != actionDomain.OrderCorrectionStatePending {
		t.Fatalf("the state word: %q", card.State)
	}
	if card.StateText != actionDomain.OrderCorrectionPendingWord {
		t.Fatalf("the state text: %q", card.StateText)
	}
	if card.Owner != correctionWireOwner.String() {
		t.Fatalf("the owner: %q", card.Owner)
	}
	if len(card.BlockedPositions) != 1 || card.BlockedPositions[0] != "WF2B196K666AA" {
		t.Fatalf("the carried blocked positions: %+v", card.BlockedPositions)
	}
	if card.BlockedReason != actionDomain.VerifyBlockReasonWrongPart {
		t.Fatalf("the carried reason word: %q", card.BlockedReason)
	}
	if !strings.Contains(card.TechnicalLine, correctionWireOper.String()) ||
		!strings.Contains(card.TechnicalLine, "ABCP-ORDER-42") ||
		!strings.Contains(card.TechnicalLine, "№1") {
		t.Fatalf("the technical line pins: %q", card.TechnicalLine)
	}
	if card.ExpectedAssignment != "1" {
		t.Fatalf("the epoch: %q", card.ExpectedAssignment)
	}
	// The template render: the section words, the THREE DIFFERENT forms
	// (AC-16) and the recommended explanation words.
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": correctionWireTenant.String(),
		"View": caseDetailPageView{CaseID: correctionWireCase.String(), OrderCorrection: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the correction panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Коррекция заказа",
		actionDomain.OrderCorrectionManualContourWord,
		actionDomain.OrderCorrectionPendingWord,
		"Записать перечит",
		"Прекратить коррекцию",
		"Переназначить коррекцию",
		actionDomain.OrderCorrectionNotSentWord,
		actionDomain.OrderCorrectionNewConfirmationWord,
		actionDomain.OrderCorrectionChannelWord,
		actionDomain.OrderCorrectionRereadWord,
		"контур 4-10",
		"Различия прежний → текущий",
		correctionWireOper.String(),
		"ABCP-ORDER-42",
	} {
		if !strings.Contains(rendered, pin) {
			t.Fatalf("the rendered page misses %q", pin)
		}
	}
	// The THREE DIFFERENT forms address DIFFERENT routes (AC-16).
	if !strings.Contains(rendered, "/order-corrections/1/rereads") ||
		!strings.Contains(rendered, "/order-corrections/1/cessations") ||
		!strings.Contains(rendered, "/order-corrections/1/reassignments") {
		t.Fatal("the three action forms ride different routes")
	}
	// The explicit former → current diff renders BY POSITION (the
	// canonical line of the pinned version against the live line).
	if !strings.Contains(rendered, "WF2B196K666AB") || !strings.Contains(rendered, "WF2B196K666AA") {
		t.Fatal("the explicit per-position diff lines render")
	}
}

// TestOrderCorrectionPanelRendersClosure pins the closure material of
// the card: the reread closure carries the SAFE pin set (the external
// object ID and the identity triple) with the reread word; the ceased
// closure of client_refused carries the cancellation recommendation
// word (AC-11 — the contour 4-10 named).
func TestOrderCorrectionPanelRendersClosure(t *testing.T) {
	states := correctionWireStates()
	reread := actionDomain.OrderCorrectionEvent{
		TenantID: correctionWireTenant, CaseID: correctionWireCase, CorrectionNo: 1, EventNo: 2,
		Kind:             actionDomain.OrderCorrectionEventReread,
		ExternalObjectID: "ABCP-ORDER-78",
		ProposalID:       id.MustParse("01994f64-6c00-7000-8000-00000000cc01"),
		VersionNo:        2, ConfirmationID: correctionWireConfirm,
		ActorID: testPrincipalID, CreatedAt: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
	}
	states[0].LastEvent = &reread
	states[0].State = actionDomain.OrderCorrectionStateCorrected
	states[0].Closure = &reread
	surface := &fakeCorrectionSurface{views: states}
	server := newCorrectionWireServer(t, surface, nil, correctionWireMaterial(), correctionWirePreviews(), correctionWireAuthorization())
	panel, err := server.orderCorrectionPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, correctionWireTenant, correctionWireCase, correctionWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	card := panel.Cards[0]
	if card.State != actionDomain.OrderCorrectionStateCorrected || card.Outcome != actionDomain.OrderCorrectionOutcomeCorrected {
		t.Fatalf("the corrected closure: %q / %q", card.State, card.Outcome)
	}
	if card.ClosureExternalID != "ABCP-ORDER-78" || card.ClosureVersionNo != "2" {
		t.Fatalf("the reread pins: %+v", card)
	}
	if card.ClosureConfirmID != correctionWireConfirm.String() {
		t.Fatalf("the confirmation pin: %q", card.ClosureConfirmID)
	}

	// The ceased closure of client_refused carries the cancellation word.
	ceased := actionDomain.OrderCorrectionEvent{
		TenantID: correctionWireTenant, CaseID: correctionWireCase, CorrectionNo: 1, EventNo: 2,
		Kind:            actionDomain.OrderCorrectionEventCeased,
		CessationReason: actionDomain.CorrectionCessationClientRefused,
		ActorID:         testPrincipalID, CreatedAt: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC),
	}
	states[0].LastEvent = &ceased
	states[0].State = actionDomain.OrderCorrectionStateCeased
	states[0].Closure = &ceased
	surface = &fakeCorrectionSurface{views: states}
	server = newCorrectionWireServer(t, surface, nil, correctionWireMaterial(), correctionWirePreviews(), correctionWireAuthorization())
	panel, err = server.orderCorrectionPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, correctionWireTenant, correctionWireCase, correctionWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	card = panel.Cards[0]
	if card.State != actionDomain.OrderCorrectionStateCeased || card.CessationReason != actionDomain.CorrectionCessationClientRefused {
		t.Fatalf("the ceased closure: %+v", card)
	}
	if card.CancellationWord != actionDomain.OrderCorrectionCancellationWord {
		t.Fatalf("the cancellation recommendation word: %q", card.CancellationWord)
	}
}

// TestOrderCorrectionPanelDegradation pins the honest degradation: an
// unwired correction surface renders the zero panel (the page keeps the
// rest); a scope-forbidden member sees the zero panel too (the
// words-first 403 rides the service ladder).
func TestOrderCorrectionPanelDegradation(t *testing.T) {
	server, err := New(fakeCorrectionWireIdentity{fakeIdentity: fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		}}}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	panel, err := server.orderCorrectionPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, correctionWireTenant, correctionWireCase, correctionWireCaseView())
	if err != nil {
		t.Fatalf("the unwired panel read fails: %v", err)
	}
	if panel.Wired || panel.Present || len(panel.Cards) != 0 {
		t.Fatalf("the unwired panel is the zero panel: %+v", panel)
	}
}

// TestOrderCorrectionOpenAPIContractMirrors pins the OpenAPI contract
// mirrors of the correction contour: the four paths, the request/outcome
// schemas, the enum mirrors of the dictionaries and the FENCE probes (no
// supplier-send field word, no change-action material — OQ-3/AG-4).
func TestOrderCorrectionOpenAPIContractMirrors(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatalf("the OpenAPI contract is unreadable: %v", err)
	}
	text := string(raw)
	for _, pin := range []string{
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-corrections:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/rereads:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/cessations:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-corrections/{correction_no}/reassignments:",
		"OrderCorrectionOpenRequest",
		"OrderCorrectionRereadRequest",
		"OrderCorrectionCeaseRequest",
		"OrderCorrectionReassignRequest",
		"OrderCorrectionOutcome",
		"enum: [correct_4h, correct_1d]",
		"enum: [client_refused, not_needed, abandoned]",
		"enum: [pending_correction, overdue, corrected, ceased]",
	} {
		if !strings.Contains(text, pin) {
			t.Fatalf("the OpenAPI contract misses %q", pin)
		}
	}
	// The supplier-send fence of the contract: no field of the
	// correction schemas names a send (the OQ-3 structural probe); the
	// change-action material stays ABSENT too (AG-4).
	for _, forbidden := range []string{"supplier_send", "send_to_supplier", "auto_send", "change_abcp_order", "modify_order"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("the contract carries a forbidden field word: %q", forbidden)
		}
	}
}
