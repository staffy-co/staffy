package httptransport

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/action/api"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityAPI "github.com/artemiosu/staffy/internal/identity/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"io"
	"log/slog"
)

// ---------------------------------------------------------------------------
// The Story 4.7 verification wire shapes: the task card rendering (the
// state word, the round material, the overdue derivation, the decision
// material with the read-time applicability), the two DIFFERENT decision
// forms (AC-16), the Commercial gate of the purchasing price, the honest
// boundary words (the manual contour, the fence honesty, the delivery
// separation) and the OpenAPI contract mirrors.
// ---------------------------------------------------------------------------

// The fixture identities of the wire world.
var (
	verificationWireTenant = id.MustParse("01994f64-6c00-7000-8000-000000000002")
	verificationWireCase   = id.MustParse("01994f64-6c00-7000-8000-00000000aa01")
	verificationWireWork   = id.MustParse("01994f64-6c00-7000-8000-00000000bb01")
	verificationWireAssign = id.MustParse("01994f64-6c00-7000-8000-00000000b201")
	verificationWireBackup = id.MustParse("01994f64-6c00-7000-8000-00000000b301")
	verificationWireOper   = id.MustParse("01994f64-6c00-7000-8000-00000000ee01")
	verificationWireDigest = strings.Repeat("f1", 32)
)

// fakeVerificationSurface is the scriptable OrderVerificationService of
// the wire tests: the read answers ride the fields, the commands answer
// zero outcomes (the command ladder is pinned by the app tests).
type fakeVerificationSurface struct {
	views       []actionDomain.OrderVerificationStateView
	material    actionDomain.OrderCommitmentMaterial
	materialErr error
}

func (f *fakeVerificationSurface) AssignOrderVerification(context.Context, api.AssignOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeVerificationSurface) AcknowledgeOrderVerification(context.Context, api.AcknowledgeOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeVerificationSurface) ApproveOrderVerification(context.Context, api.ApproveOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeVerificationSurface) BlockOrderVerification(context.Context, api.BlockOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeVerificationSurface) ReassignOrderVerification(context.Context, api.ReassignOrderVerificationCommand) (api.OrderVerificationOutcome, error) {
	return api.OrderVerificationOutcome{}, nil
}
func (f *fakeVerificationSurface) ReadOrderVerificationCaseView(context.Context, identityDomain.Principal, id.UUID, id.UUID, time.Time) ([]actionDomain.OrderVerificationStateView, error) {
	return f.views, nil
}
func (f *fakeVerificationSurface) ReadOrderVerificationLiveMaterial(context.Context, identityDomain.Principal, id.UUID, id.UUID) (actionDomain.OrderCommitmentMaterial, error) {
	return f.material, f.materialErr
}

// fakeVerificationPreviews serves the preview cards of the order thread
// (the pinned render material of the diff rows).
type fakeVerificationPreviews struct {
	api.PreviewService
	view api.PreviewView
}

func (f fakeVerificationPreviews) ReadPreviewView(context.Context, identityDomain.Principal, id.UUID, id.UUID) (api.PreviewView, error) {
	return f.view, nil
}

// fakeVerificationAuthorization serves the operation cards of the
// technical line.
type fakeVerificationAuthorization struct {
	api.AuthorizationService
	view api.AuthorizationView
}

func (f fakeVerificationAuthorization) ReadAuthorizationView(context.Context, identityDomain.Principal, id.UUID, id.UUID, time.Time) (api.AuthorizationView, error) {
	return f.view, nil
}

// fakeVerificationIdentity adds the membership scope read over the wire
// fake identity (the Commercial gate of the purchasing price).
type fakeVerificationIdentity struct {
	fakeIdentity
	commercial bool
}

func (f fakeVerificationIdentity) Authorize(_ context.Context, _ identityDomain.Principal, _ id.UUID, required identityDomain.Scope) error {
	if required == identityDomain.Commercial && f.commercial {
		return nil
	}
	if required == identityDomain.Commercial {
		return identityDomain.ErrForbidden
	}
	return nil
}

// newVerificationWireServer wires the server of the wire tests over the
// scriptable surfaces.
func newVerificationWireServer(t *testing.T, surface *fakeVerificationSurface, previews api.PreviewView, authorization api.AuthorizationView, commercial bool) *Server {
	t.Helper()
	identity := fakeVerificationIdentity{fakeIdentity: fakeIdentity{
		create: func(identityAPI.CreateTenantCommand) (identityAPI.CreateTenantResult, error) {
			return identityAPI.CreateTenantResult{}, nil
		}, workspace: func(id.UUID) (identityDomain.Workspace, error) {
			return identityDomain.Workspace{}, nil
		}}, commercial: commercial}
	server, err := New(identity, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	server.orderVerifications = surface
	server.actionPreviews = fakeVerificationPreviews{view: previews}
	server.authorization = fakeVerificationAuthorization{view: authorization}
	return server
}

// verificationWireTasks builds the acknowledged task view with the
// approved decision material.
func verificationWireTasks() []actionDomain.OrderVerificationStateView {
	task := actionDomain.OrderVerification{
		TenantID: verificationWireTenant, CaseID: verificationWireCase, WorkItemID: verificationWireWork,
		VerificationNo: 1, OperationID: verificationWireOper,
		ReceiptVersionNo: 1, ExternalOrderID: "ABCP-ORDER-42",
		CreatorID: testPrincipalID, CreatedAt: time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
	}
	assigned := actionDomain.OrderVerificationEvent{
		TenantID: verificationWireTenant, CaseID: verificationWireCase, VerificationNo: 1, EventNo: 1,
		Kind:     actionDomain.OrderVerificationEventAssigned,
		Assignee: verificationWireAssign, BackupOwner: verificationWireBackup,
		VerifyDeadline: actionDomain.Verify4h, DeadlineAt: time.Date(2026, 10, 25, 13, 0, 0, 0, time.UTC),
		ActorID: testPrincipalID, CreatedAt: time.Date(2026, 10, 25, 9, 5, 0, 0, time.UTC),
	}
	acknowledged := assigned
	acknowledged.Kind = actionDomain.OrderVerificationEventAcknowledged
	acknowledged.EventNo = 2
	return []actionDomain.OrderVerificationStateView{{
		Verification: task, LastEvent: &acknowledged,
		State:    actionDomain.OrderVerificationStateAcknowledged,
		Assignee: verificationWireAssign, BackupOwner: verificationWireBackup,
		VerifyDeadline: actionDomain.Verify4h, DeadlineAt: assigned.DeadlineAt, AssignmentEpoch: 1,
	}}
}

// verificationWireMaterial builds the live material of one position.
func verificationWireMaterial() actionDomain.OrderCommitmentMaterial {
	return actionDomain.OrderCommitmentMaterial{
		ProfileDigestPin:     strings.Repeat("a", 64),
		PartRequestDigestPin: strings.Repeat("b", 64),
		Positions: []actionDomain.OrderPosition{{
			PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 3,
			Brand: "Mann", Article: "WF2B196K666AA", Originality: actionDomain.OrderOriginalityOriginal,
			Quantity: 2, UnitClientPrice: 4200, Currency: "RUB",
			FitmentDigestPin: strings.Repeat("c", 64), OfferDigestPin: strings.Repeat("d", 64),
			SelectionDigest: strings.Repeat("e", 64),
		}},
		KnownClientSum: 8400, Currency: "RUB",
	}
}

// verificationWirePreviews builds the preview view carrying the order
// card with the typed input (the pinned render — DIVERGES from the live
// material: the price moved after the confirmation, the diff row renders).
func verificationWirePreviews() api.PreviewView {
	pinned := verificationWireMaterial()
	pinned.Positions[0].UnitClientPrice = 4300
	pinned.KnownClientSum = 8600
	material := actionDomain.OrderMaterialCanonicalText(pinned)
	card := api.PreviewCardView{
		PreviewID:         id.MustParse("01994f64-6c00-7000-8000-00000000cc01"),
		VersionNo:         1,
		DefinitionID:      actionDomain.OrderActionID,
		DefinitionVersion: actionDomain.OrderActionVersion,
		CaseID:            verificationWireCase,
		WorkItemID:        verificationWireWork,
		TypedInput:        map[string]string{"order_digest": verificationWireDigest, "order_material": material, "known_client_sum": "8400.00", "currency": "RUB"},
		Current:           true,
	}
	return api.PreviewView{TenantID: verificationWireTenant, CaseID: verificationWireCase,
		Threads: []api.PreviewThreadView{{PreviewID: card.PreviewID, Cards: []api.PreviewCardView{card}}}}
}

// verificationWireAuthorization builds the operation card with the
// committed receipt of the technical line.
func verificationWireAuthorization() api.AuthorizationView {
	receipt := api.ActionReceiptView{
		ReceiptVersionNo: 1, Outcome: actionDomain.ReceiptOutcomeCommitted,
		ExternalObjectID: "ABCP-ORDER-42", ExternalObjectVersion: "3",
	}
	return api.AuthorizationView{TenantID: verificationWireTenant, CaseID: verificationWireCase,
		Operations: []api.OperationCardView{{
			OperationID: verificationWireOper, DefinitionID: actionDomain.OrderActionID,
			PreviewID: id.MustParse("01994f64-6c00-7000-8000-00000000cc01"), PreviewVersionNo: 1,
			CaseID: verificationWireCase, WorkItemID: verificationWireWork,
			IdempotencyKey: "order-key-0001", Receipts: []api.ActionReceiptView{receipt},
		}}}
}

// verificationWireCaseView builds the case view with the purchasing price
// of the current offer record (the Commercial gate material: the visible
// actor reads the internal data word, the hidden actor reads the hiding
// word).
func verificationWireCaseView() casesAPI.CaseView {
	purchase := 3900.50
	return casesAPI.CaseView{Comparison: casesAPI.ComparisonPanelView{
		SchemaDefined: true,
		Records: []casesDomain.OfferView{{
			OfferNo: 3, PositionKey: "WF2B196K666AA", SupplierID: "SUP-2",
			Brand: "Mann", Article: "WF2B196K666AA",
			ClientPrice: 4200, PurchasePrice: &purchase, Currency: "RUB",
			OfferDigest: strings.Repeat("d", 64),
		}},
	}}
}

// TestOrderVerificationPanelRendersTheTaskCard pins the task card
// rendering: the state word with its honest text, the round material (the
// assignee and the backup by id, the deadline word with its time), the
// assignment epoch, the technical line (the operation, the idempotency
// key, the external ABCP Order ID, the 1C-unknown word) and the honest
// boundary words of the section (the delivery separation, the fence
// honesty).
func TestOrderVerificationPanelRendersTheTaskCard(t *testing.T) {
	surface := &fakeVerificationSurface{views: verificationWireTasks(), material: verificationWireMaterial()}
	server := newVerificationWireServer(t, surface, verificationWirePreviews(), verificationWireAuthorization(), true)
	panel, err := server.orderVerificationPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, verificationWireTenant, verificationWireCase, verificationWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if !panel.Present || len(panel.Cards) != 1 {
		t.Fatalf("the task card is present: %+v", panel)
	}
	card := panel.Cards[0]
	if card.State != actionDomain.OrderVerificationStateAcknowledged {
		t.Fatalf("the state word: %q", card.State)
	}
	if !strings.Contains(card.TechnicalLine, verificationWireOper.String()) ||
		!strings.Contains(card.TechnicalLine, "order-key-0001") ||
		!strings.Contains(card.TechnicalLine, "ABCP-ORDER-42") {
		t.Fatalf("the technical line pins: %q", card.TechnicalLine)
	}
	if card.Assignee != verificationWireAssign.String() || card.BackupOwner != verificationWireBackup.String() {
		t.Fatalf("the round material: %q / %q", card.Assignee, card.BackupOwner)
	}
	if card.ExpectedAssignment != "1" {
		t.Fatalf("the epoch: %q", card.ExpectedAssignment)
	}
	if !panel.LivePresent || panel.LiveDigest != actionDomain.OrderDigestOf(verificationWireMaterial()) {
		t.Fatalf("the live digest: %q", panel.LiveDigest)
	}
	// The template render: the section words and the two DIFFERENT
	// decision forms (AC-16).
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": verificationWireTenant.String(),
		"View": caseDetailPageView{CaseID: verificationWireCase.String(), OrderVerification: panel}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the verification panel: %v", err)
	}
	rendered := page.String()
	for _, pin := range []string{
		"Проверка заказа",
		actionDomain.OrderVerificationDeliveryWord,
		actionDomain.OrderVerificationFenceWord,
		actionDomain.OrderVerificationManualContourWord,
		"Подтвердить проверку",
		"Нужна коррекция",
		actionDomain.OrderDigestOf(verificationWireMaterial()),
		"Явные различия",
		"ABCP-ORDER-42",
		actionDomain.OrderOneCUnknownWord,
	} {
		if !strings.Contains(rendered, pin) {
			t.Fatalf("the rendered page misses %q", pin)
		}
	}
	// The two DIFFERENT forms address DIFFERENT routes (AC-16).
	if !strings.Contains(rendered, "/order-verifications/1/approvals") || !strings.Contains(rendered, "/order-verifications/1/blocks") {
		t.Fatal("the two decision forms ride different routes")
	}
}

// TestOrderVerificationPanelHidesPurchasingWithoutCommercial pins the
// Commercial gate (AC-4): the actor without the scope sees the honest
// hiding word instead of the purchasing price; the Commercial actor sees
// the internal data word of the recorded price.
func TestOrderVerificationPanelHidesPurchasingWithoutCommercial(t *testing.T) {
	surface := &fakeVerificationSurface{views: verificationWireTasks(), material: verificationWireMaterial()}
	// The hidden actor.
	server := newVerificationWireServer(t, surface, verificationWirePreviews(), verificationWireAuthorization(), false)
	panel, err := server.orderVerificationPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, verificationWireTenant, verificationWireCase, verificationWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if !panel.PurchasingHidden {
		t.Fatal("the purchasing price is hidden without the Commercial scope")
	}
	if len(panel.LivePositions) != 1 || panel.LivePositions[0].PurchasingText != actionDomain.OrderVerificationPurchasingHiddenWord {
		t.Fatalf("the honest hiding word: %q", panel.LivePositions[0].PurchasingText)
	}
	// The Commercial actor.
	serverCommercial := newVerificationWireServer(t, surface, verificationWirePreviews(), verificationWireAuthorization(), true)
	panelCommercial, err := serverCommercial.orderVerificationPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, verificationWireTenant, verificationWireCase, verificationWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if panelCommercial.PurchasingHidden {
		t.Fatal("the Commercial actor sees the purchasing material")
	}
	if len(panelCommercial.LivePositions) != 1 || !strings.Contains(panelCommercial.LivePositions[0].PurchasingText, "внутренние данные") {
		t.Fatalf("the purchasing word of the Commercial actor: %q", panelCommercial.LivePositions[0].PurchasingText)
	}
}

// TestOrderVerificationPanelRendersOverdueAndDecision pins the overdue
// derivation (the honest word with the reassignment path) and the
// decision material with the read-time applicability word of a stale
// approved digest (AC-14).
func TestOrderVerificationPanelRendersOverdueAndDecision(t *testing.T) {
	views := verificationWireTasks()
	// The overdue round: the deadline passed without a closing event.
	views[0].State = actionDomain.OrderVerificationStateOverdue
	surface := &fakeVerificationSurface{views: views, material: verificationWireMaterial()}
	server := newVerificationWireServer(t, surface, verificationWirePreviews(), verificationWireAuthorization(), true)
	panel, err := server.orderVerificationPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, verificationWireTenant, verificationWireCase, verificationWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if !panel.Cards[0].Overdue || panel.Cards[0].OverdueWord != actionDomain.OrderVerificationOverdueWord {
		t.Fatalf("the overdue word: %+v", panel.Cards[0])
	}
	// The approved decision over a STALE digest: the read-time
	// inapplicability word (AC-14).
	approved := views[0]
	approved.State = actionDomain.OrderVerificationStateApproved
	approved.LastEvent = &actionDomain.OrderVerificationEvent{
		Kind:        actionDomain.OrderVerificationEventApproved,
		OrderDigest: strings.Repeat("aa", 32), ReceiptVersionNo: 1,
		ExternalOrderID: "ABCP-ORDER-42",
		EvidenceAnchors: []string{"position:WF2B196K666AA"},
	}
	approved.Decision = approved.LastEvent
	surfaceApproved := &fakeVerificationSurface{views: []actionDomain.OrderVerificationStateView{approved}, material: verificationWireMaterial()}
	serverApproved := newVerificationWireServer(t, surfaceApproved, verificationWirePreviews(), verificationWireAuthorization(), true)
	panelApproved, err := serverApproved.orderVerificationPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, verificationWireTenant, verificationWireCase, verificationWireCaseView())
	if err != nil {
		t.Fatalf("the panel read fails: %v", err)
	}
	if panelApproved.Cards[0].Applicable {
		t.Fatal("the stale digest marks the former decision inapplicable")
	}
	if panelApproved.Cards[0].ApplicabilityWord != actionDomain.OrderVerificationInapplicableWord {
		t.Fatalf("the inapplicability word: %q", panelApproved.Cards[0].ApplicabilityWord)
	}
	if len(panelApproved.DiffRows) == 0 {
		t.Fatal("the explicit field differences render for the stale composition")
	}
	// The approved decision card renders the evidence anchors (the SAFE
	// pin set of the Human Verification Receipt).
	templates := decisionContextRenderTemplates(t)
	var page bytes.Buffer
	data := map[string]any{"TenantID": verificationWireTenant.String(),
		"View": caseDetailPageView{CaseID: verificationWireCase.String(), OrderVerification: panelApproved}}
	if err := templates.ExecuteTemplate(&page, "case", data); err != nil {
		t.Fatalf("the case page does not render with the approved card: %v", err)
	}
	for _, pin := range []string{"position:WF2B196K666AA", actionDomain.OrderVerificationInapplicableWord, actionDomain.OrderVerificationManualContourWord} {
		if !strings.Contains(page.String(), pin) {
			t.Fatalf("the approved card render misses %q", pin)
		}
	}
}

// TestOrderVerificationPanelDegradationAndRefusalWords pins the honest
// degradation: the unwired surface renders the zero panel; the live
// derivation refusal is SPOKEN (the honest word, never an empty context).
func TestOrderVerificationPanelDegradationAndRefusalWords(t *testing.T) {
	// The unwired surface: the zero panel, the page keeps the rest.
	server, err := New(fakeIdentity{}, fixedResolver{principal: identityDomain.Principal{ID: testPrincipalID}}, fixedIDs{}, fixedClock{}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	panel, err := server.orderVerificationPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, verificationWireTenant, verificationWireCase, verificationWireCaseView())
	if err != nil {
		t.Fatalf("the unwired surface degrades honestly: %v", err)
	}
	if panel.Present || len(panel.Cards) != 0 {
		t.Fatalf("the zero panel of the unwired surface: %+v", panel)
	}
	// The derivation refusal is spoken.
	surface := &fakeVerificationSurface{views: verificationWireTasks(), materialErr: actionDomain.ErrOrderNoSelection}
	serverWired := newVerificationWireServer(t, surface, verificationWirePreviews(), verificationWireAuthorization(), true)
	panelRefusal, err := serverWired.orderVerificationPanelOf(context.Background(), identityDomain.Principal{ID: testPrincipalID}, verificationWireTenant, verificationWireCase, verificationWireCaseView())
	if err != nil {
		t.Fatalf("the refusal panel read fails: %v", err)
	}
	if panelRefusal.LiveRefusalWord != actionDomain.ErrOrderNoSelection.Error() {
		t.Fatalf("the spoken refusal word: %q", panelRefusal.LiveRefusalWord)
	}
}

// TestOrderVerificationOpenAPIContractMirrors pins the OpenAPI mirrors of
// the five paths and the closed-shape schemas with the enum dictionaries
// (drift turns this red — the contract test of the frozen words).
func TestOrderVerificationOpenAPIContractMirrors(t *testing.T) {
	raw, err := os.ReadFile("../../../contracts/openapi/staffy.yaml")
	if err != nil {
		t.Fatalf("the OpenAPI contract is unreadable: %v", err)
	}
	text := string(raw)
	for _, pin := range []string{
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-verifications:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/acknowledgements:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/approvals:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/blocks:",
		"/v1/tenants/{tenant_id}/cases/{case_id}/order-verifications/{verification_no}/reassignments:",
		"OrderVerificationAssignRequest",
		"OrderVerificationEpochRequest",
		"OrderVerificationApproveRequest",
		"OrderVerificationBlockRequest",
		"OrderVerificationReassignRequest",
		"OrderVerificationOutcome",
		"enum: [verify_4h, verify_1d]",
		"enum: [wrong_part, wrong_vehicle, price_changed, availability_changed, confirmation_issue]",
		"enum: [assigned, acknowledged, approved, blocked, overdue]",
	} {
		if !strings.Contains(text, pin) {
			t.Fatalf("the OpenAPI contract misses %q", pin)
		}
	}
	// The supplier-send fence of the contract: no field of the
	// verification schemas names a send (the OQ-3 structural probe).
	for _, forbidden := range []string{"supplier_send", "send_to_supplier", "auto_send"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("the contract carries a supplier-send field word: %q", forbidden)
		}
	}
}
