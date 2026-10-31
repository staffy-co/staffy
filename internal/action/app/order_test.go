package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/staffy-co/staffy/internal/action/api"
	actionCatalog "github.com/staffy-co/staffy/internal/action/catalog"
	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/action/port"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	connectorAPI "github.com/staffy-co/staffy/internal/connector/api"
	connectorDomain "github.com/staffy-co/staffy/internal/connector/domain"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.6 step-02: the order commitment builder (the live derivation)
// ---------------------------------------------------------------------------

// orderTestPin derives a deterministic 64-hex pin for the fixtures (the
// SHA-256 hex of the label — a valid digest word).
func orderTestPin(label string) string {
	sum := sha256.Sum256([]byte("staffy-order-app-test:" + label))
	return hex.EncodeToString(sum[:])
}

// orderCaseView builds the case view of ONE selected-and-proven position:
// the vehicle and part request panels with the LIVE case-level digests,
// the proven fitment with the LIVE FitmentDigest, the offer records (the
// CURRENT record of the chosen supplier) and ONE recorded selection
// snapshot pinned to the same versions.
func orderCaseView() casesAPI.CaseView {
	view := newPreviewLiveFixture().caseView
	view.Vehicle = casesAPI.VehiclePanelView{
		SchemaDefined: true,
		Profile:       casesDomain.VehicleProfileView{ProfileDigest: orderTestPin("profile")},
	}
	view.PartRequest = casesAPI.PartPanelView{
		SchemaDefined: true,
		Part:          casesDomain.PartRequestView{PartRequestDigest: orderTestPin("partreq")},
	}
	view.Fitment = casesAPI.FitmentPanelView{
		SchemaDefined: true,
		Fitment: casesDomain.FitmentView{Positions: []casesDomain.FitmentPositionView{{
			PositionKey:   "pos-1",
			Status:        casesDomain.FitmentStatusProven,
			FitmentDigest: orderTestPin("fitment"),
		}}},
	}
	view.Comparison = casesAPI.ComparisonPanelView{
		SchemaDefined: true,
		Comparison: casesDomain.ComparisonView{Positions: []casesDomain.ComparisonPositionView{{
			PositionKey:   "pos-1",
			LinkType:      casesDomain.FitmentLinkOriginal,
			FitmentStatus: casesDomain.FitmentStatusProven,
			Status:        casesDomain.OfferStatusCurrent,
		}}},
		Records: []casesDomain.OfferView{{
			OfferNo:     3,
			PositionKey: "pos-1",
			SupplierID:  "supplier-a",
			Brand:       "Bosch",
			Article:     "341167866712",
			ClientPrice: 4200.50,
			Currency:    "RUB",
			OfferDigest: orderTestPin("offer"),
		}},
		Selections: []casesDomain.SelectionSnapshotView{{
			SnapshotNo:           1,
			PositionKey:          "pos-1",
			SupplierID:           "supplier-a",
			OfferNo:              3,
			Quantity:             2,
			UnitClientPrice:      4200.50,
			Currency:             "RUB",
			KnownClientSum:       8401.00,
			ProfileDigestPin:     orderTestPin("profile"),
			PartRequestDigestPin: orderTestPin("partreq"),
			FitmentDigestPin:     orderTestPin("fitment"),
			OfferDigestPin:       orderTestPin("offer"),
			SelectionDigest:      orderTestPin("select"),
		}},
	}
	return view
}

// TestDeriveOrderMaterialNoSelectionRefuses — нет записанного выбора —
// честный отказ словом ДО какого-либо материала.
func TestDeriveOrderMaterialNoSelectionRefuses(t *testing.T) {
	view := orderCaseView()
	view.Comparison.Selections = nil
	_, err := DeriveOrderCommitmentMaterial(view)
	if !errors.Is(err, domain.ErrOrderNoSelection) {
		t.Fatalf("нет выбора — отказ ErrOrderNoSelection, получено: %v", err)
	}
}

// TestDeriveOrderMaterialAssemblesLive — валидный выбор собирает материал
// живьём: позиция с пинами трёх входов, бренд/артикул из записи предложения,
// оригинальность из типа связи, количество/цена/сумма из снапшота выбора.
func TestDeriveOrderMaterialAssemblesLive(t *testing.T) {
	material, err := DeriveOrderCommitmentMaterial(orderCaseView())
	if err != nil {
		t.Fatalf("валидный выбор обязан собирать материал: %v", err)
	}
	if material.ProfileDigestPin != orderTestPin("profile") || material.PartRequestDigestPin != orderTestPin("partreq") {
		t.Fatal("пины кейс-уровня обязаны прийти из живого снапшота выбора")
	}
	if len(material.Positions) != 1 {
		t.Fatalf("одна позиция, получено %d", len(material.Positions))
	}
	position := material.Positions[0]
	if position.Brand != "Bosch" || position.Article != "341167866712" {
		t.Fatalf("бренд/артикул из записи предложения: %s / %s", position.Brand, position.Article)
	}
	if position.Originality != domain.OrderOriginalityOriginal {
		t.Fatalf("оригинальность из типа связи original: %q", position.Originality)
	}
	if position.Quantity != 2 || position.UnitClientPrice != 4200.50 {
		t.Fatalf("количество/цена из выбора: %d / %f", position.Quantity, position.UnitClientPrice)
	}
	if position.FitmentDigestPin != orderTestPin("fitment") ||
		position.OfferDigestPin != orderTestPin("offer") ||
		position.SelectionDigest != orderTestPin("select") {
		t.Fatal("пины применимости/предложения/выбора обязаны приехать из снапшота")
	}
	if material.KnownClientSum != 8401.00 {
		t.Fatalf("известная сумма — арифметика выбора: %f", material.KnownClientSum)
	}
	if material.Currency != "RUB" {
		t.Fatalf("валюта из выбора: %q", material.Currency)
	}
}

// TestDeriveOrderMaterialCrossLinkIsAnalog — тип связи cross → слово
// аналога словаря заказа.
func TestDeriveOrderMaterialCrossLinkIsAnalog(t *testing.T) {
	view := orderCaseView()
	view.Comparison.Comparison.Positions[0].LinkType = casesDomain.FitmentLinkCross
	material, err := DeriveOrderCommitmentMaterial(view)
	if err != nil {
		t.Fatalf("кросс-связь обязана собираться: %v", err)
	}
	if material.Positions[0].Originality != domain.OrderOriginalityAnalog {
		t.Fatalf("кросс-связь = аналог: %q", material.Positions[0].Originality)
	}
}

// TestDeriveOrderMaterialForeignLinkRefuses — чужой тип связи отказывается
// fail-closed (словарь оригинальности — закрытый).
func TestDeriveOrderMaterialForeignLinkRefuses(t *testing.T) {
	view := orderCaseView()
	view.Comparison.Comparison.Positions[0].LinkType = "universal"
	if _, err := DeriveOrderCommitmentMaterial(view); err == nil {
		t.Fatal("чужой тип связи обязан отказываться (fail-closed словарь оригинальности)")
	}
}

// TestDeriveOrderMaterialNonProvenRefuses — выбор позиции с non-proven
// применимостью — честный отказ словом применимости.
func TestDeriveOrderMaterialNonProvenRefuses(t *testing.T) {
	view := orderCaseView()
	view.Comparison.Comparison.Positions[0].FitmentStatus = casesDomain.FitmentStatusStale
	_, err := DeriveOrderCommitmentMaterial(view)
	if !errors.Is(err, domain.ErrOrderFitmentNotProven) {
		t.Fatalf("non-proven применимость — отказ ErrOrderFitmentNotProven, получено: %v", err)
	}
}

// TestDeriveOrderMaterialMissingFitmentPositionRefuses — позиция выбора без
// живой панели применимости — тот же честный отказ.
func TestDeriveOrderMaterialMissingFitmentPositionRefuses(t *testing.T) {
	view := orderCaseView()
	view.Comparison.Comparison.Positions = nil
	_, err := DeriveOrderCommitmentMaterial(view)
	if !errors.Is(err, domain.ErrOrderFitmentNotProven) {
		t.Fatalf("отсутствующая позиция применимости — отказ ErrOrderFitmentNotProven, получено: %v", err)
	}
}

// TestDeriveOrderMaterialMixedPinsRefuses — выбор, запинненный на СТАРУЮ
// версию входа (новая версия профиля после выбора) — честный отказ
// смешанных версий.
func TestDeriveOrderMaterialMixedPinsRefuses(t *testing.T) {
	view := orderCaseView()
	view.Vehicle.Profile.ProfileDigest = orderTestPin("profilenew")
	_, err := DeriveOrderCommitmentMaterial(view)
	if !errors.Is(err, domain.ErrOrderMixedInputVersions) {
		t.Fatalf("выбор под старой версией входа — отказ ErrOrderMixedInputVersions, получено: %v", err)
	}
}

// TestDeriveOrderMaterialLivePriceChangesTheDigest — ТЕКУЩАЯ запись
// предложения (новая цена перезаписью) меняет материал живьём: цена и
// сумма читаются из текущей записи, дайджест позиции — текущий OfferDigest
// (execute-time revalidation не слепа к изменению цены, AC-5).
func TestDeriveOrderMaterialLivePriceChangesTheDigest(t *testing.T) {
	view := orderCaseView()
	base, err := DeriveOrderCommitmentMaterial(view)
	if err != nil {
		t.Fatalf("базовый материал обязан собираться: %v", err)
	}
	// The supplier re-records the offer: a NEW offer number with the new
	// price — the CURRENT record of the pair moves on.
	view.Comparison.Records = append(view.Comparison.Records, casesDomain.OfferView{
		OfferNo: 4, PositionKey: "pos-1", SupplierID: "supplier-a",
		Brand: "Bosch", Article: "341167866712",
		ClientPrice: 4900, Currency: "RUB", OfferDigest: orderTestPin("offernew"),
	})
	changed, err := DeriveOrderCommitmentMaterial(view)
	if err != nil {
		t.Fatalf("переписанное предложение обязано собираться: %v", err)
	}
	if changed.Positions[0].UnitClientPrice != 4900 || changed.KnownClientSum != 9800 {
		t.Fatalf("цена/сумма из ТЕКУЩЕЙ записи: цена %f, сумма %f", changed.Positions[0].UnitClientPrice, changed.KnownClientSum)
	}
	if changed.Positions[0].OfferNo != 4 || changed.Positions[0].OfferDigestPin != orderTestPin("offernew") {
		t.Fatalf("позиция несёт текущую запись: №%d, дайджест %q", changed.Positions[0].OfferNo, changed.Positions[0].OfferDigestPin)
	}
	if domain.OrderDigestOf(changed) == domain.OrderDigestOf(base) {
		t.Fatal("изменение цены обязано менять дайджест — revalidation не слепа")
	}
}

// TestDeriveOrderMaterialMixedCurrencyRefuses — выборы в разных валютах —
// честный отказ (один заказ — одна валюта).
func TestDeriveOrderMaterialMixedCurrencyRefuses(t *testing.T) {
	view := orderCaseView()
	view.Comparison.Selections = append(view.Comparison.Selections, casesDomain.SelectionSnapshotView{
		SnapshotNo:           2,
		PositionKey:          "pos-2",
		SupplierID:           "supplier-b",
		OfferNo:              1,
		Quantity:             1,
		UnitClientPrice:      100,
		Currency:             "EUR",
		KnownClientSum:       100,
		ProfileDigestPin:     orderTestPin("profile"),
		PartRequestDigestPin: orderTestPin("partreq"),
		FitmentDigestPin:     orderTestPin("fitment"),
		OfferDigestPin:       orderTestPin("offer"),
		SelectionDigest:      orderTestPin("select"),
	})
	_, err := DeriveOrderCommitmentMaterial(view)
	if !errors.Is(err, domain.ErrOrderMixedCurrency) {
		t.Fatalf("смешанные валюты — отказ ErrOrderMixedCurrency, получено: %v", err)
	}
}

// TestDeriveOrderMaterialOfferUnresolvedRefuses — пин предложения не
// резолвится против живых записей — отказ целостности.
func TestDeriveOrderMaterialOfferUnresolvedRefuses(t *testing.T) {
	view := orderCaseView()
	view.Comparison.Records = nil
	_, err := DeriveOrderCommitmentMaterial(view)
	if !errors.Is(err, domain.ErrOrderOfferUnresolved) {
		t.Fatalf("нерезолвируемое предложение — отказ ErrOrderOfferUnresolved, получено: %v", err)
	}
}

// TestDeriveOrderMaterialReselectionSupersedes — повторный выбор той же
// позиции (новый снапшот) заменяет старый: берётся ПОСЛЕДНИЙ снапшот.
func TestDeriveOrderMaterialReselectionSupersedes(t *testing.T) {
	view := orderCaseView()
	reselection := view.Comparison.Selections[0]
	reselection.SnapshotNo = 2
	reselection.Quantity = 5
	reselection.UnitClientPrice = 4200.50
	reselection.KnownClientSum = 21002.50
	reselection.SelectionDigest = orderTestPin("selectnew")
	view.Comparison.Selections = append(view.Comparison.Selections, reselection)
	material, err := DeriveOrderCommitmentMaterial(view)
	if err != nil {
		t.Fatalf("повторный выбор обязан собираться: %v", err)
	}
	if len(material.Positions) != 1 {
		t.Fatalf("одна позиция после повторного выбора, получено %d", len(material.Positions))
	}
	if material.Positions[0].Quantity != 5 || material.KnownClientSum != 21002.50 {
		t.Fatalf("последний снапшот побеждает: количество %d, сумма %f", material.Positions[0].Quantity, material.KnownClientSum)
	}
	if material.Positions[0].SelectionDigest != orderTestPin("selectnew") {
		t.Fatal("пин выбора — из последнего снапшота")
	}
}

// ---------------------------------------------------------------------------
// PrepareOrderProposal — the builder over the REAL product entry
// ---------------------------------------------------------------------------

// orderConnectors serves the ABCP capability facts of the order definition
// (the fixture variant of fakeConnectors over abcp-parts).
type orderConnectors struct{ state, contractVersion string }

func (f orderConnectors) ConnectionsFacts(_ context.Context, _ tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
	return []connectorAPI.ConnectionFacts{{
		ConnectionID: previewConnID, ConnectorID: "abcp-parts",
		ContractVersion: f.contractVersion,
		Capabilities: []connectorDomain.CapabilityReport{{
			CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "command.prepare-client-order"},
			State:                 f.state,
		}},
	}}, nil
}

// orderWorld builds the PASSING live world of the order action: the
// operator role, the sales mode, the OQ-3/AG-4 gate passes, the synthetic
// stage, the verified ABCP capability and the case view with the recorded
// selection.
func orderWorld() previewLiveFixture {
	world := newPreviewLiveFixture()
	world.roles = []identityDomain.Role{identityDomain.Operator}
	world.instances = []employeeDomain.GateInstance{
		{EmployeeID: previewEmployee, DefinitionID: "OQ-3", State: "pass", RecordedAt: previewNow},
		{EmployeeID: previewEmployee, DefinitionID: "AG-4", State: "pass", RecordedAt: previewNow},
	}
	world.caseView = orderCaseView()
	world.caseView.Mode = casesAPI.ModeView{Active: &casesDomain.ModeTerm{
		TenantID: testTenant, CaseID: previewCaseID, Mode: "sales", ModeVersion: 1,
	}}
	return world
}

// newOrderBuilderService wires the REAL product catalog entry over the
// order world (the unit composition root of the builder tests).
func newOrderBuilderService(world previewLiveFixture) *Service {
	return New(previewIdentity(), acceptAllPacks{}, acceptAllCaps{}, actionCatalog.All()).
		WithPreviews(newFakePreviewRepo(), world.control, fakeCaseMaterial{view: world.caseView, facts: world.factsView},
			liveMembers{world: &world}, fakeEntitlement{status: world.admission},
			fakeReadiness{stage: world.stage, instances: world.instances},
			orderConnectors{state: world.capState, contractVersion: world.contract},
			fakeRouteFacts{profileVersion: world.profile}, &fakeIDs{}).
		WithPreviewReader(world.reader).
		WithPreviewDryRunner(world.dryRunner)
}

// TestPrepareOrderProposalBuildsThePreviewOverTheRealEntry — билдер
// собирает материал живьём и подаёт предложение машины 3-4 с typed input
// из определения: вердикт valid, версия построена, digest 64-hex.
// Команда НЕ несёт НИ ОДНОГО материала — зонд «оператор не вводит пины».
func TestPrepareOrderProposalBuildsThePreviewOverTheRealEntry(t *testing.T) {
	world := orderWorld()
	service := newOrderBuilderService(world)
	outcome, err := service.PrepareOrderProposal(context.Background(), api.PrepareOrderProposalCommand{
		Actor:      testActor,
		TenantID:   testTenant,
		CaseID:     previewCaseID,
		WorkItemID: previewWorkID,
		Now:        previewNow,
	})
	if err != nil {
		t.Fatalf("билдер обязан подать предложение через машину 3-4: %v", err)
	}
	if outcome.Verdict != domain.VerdictValid {
		t.Fatalf("вердикт valid, получено %q (нарушения: %v; блок: %q)", outcome.Verdict, outcome.Violations, outcome.BlockedReason)
	}
	if outcome.PreviewID == "" || outcome.VersionNo != 1 {
		t.Fatalf("версия 1 построена: %+v", outcome)
	}
	if len(outcome.PreviewDigest) != 64 {
		t.Fatalf("digest.preview 64-hex: %q", outcome.PreviewDigest)
	}
}

// TestPrepareOrderProposalNoSelectionRefusesBeforeAnyRow — нет выбора —
// честный отказ ДО записи попытки (попыток нет в репозитории).
func TestPrepareOrderProposalNoSelectionRefusesBeforeAnyRow(t *testing.T) {
	world := orderWorld()
	world.caseView.Comparison.Selections = nil
	repo := newFakePreviewRepo()
	service := New(previewIdentity(), acceptAllPacks{}, acceptAllCaps{}, actionCatalog.All()).
		WithPreviews(repo, world.control, fakeCaseMaterial{view: world.caseView, facts: world.factsView},
			liveMembers{world: &world}, fakeEntitlement{status: world.admission},
			fakeReadiness{stage: world.stage, instances: world.instances},
			orderConnectors{state: world.capState, contractVersion: world.contract},
			fakeRouteFacts{profileVersion: world.profile}, &fakeIDs{}).
		WithPreviewReader(world.reader).
		WithPreviewDryRunner(world.dryRunner)
	_, err := service.PrepareOrderProposal(context.Background(), api.PrepareOrderProposalCommand{
		Actor:      testActor,
		TenantID:   testTenant,
		CaseID:     previewCaseID,
		WorkItemID: previewWorkID,
		Now:        previewNow,
	})
	if !errors.Is(err, domain.ErrOrderNoSelection) {
		t.Fatalf("нет выбора — отказ словом ДО предложения, получено: %v", err)
	}
	if len(repo.attempts) != 0 {
		t.Fatalf("отказ билдера НЕ пишет попыток, записано %d", len(repo.attempts))
	}
}

// TestPrepareOrderProposalNonProvenRefuses — non-proven применимость —
// честный отказ билдера (guard 4-5 уже отказал запись; билдер
// ре-выводит живьём).
func TestPrepareOrderProposalNonProvenRefuses(t *testing.T) {
	world := orderWorld()
	world.caseView.Comparison.Comparison.Positions[0].FitmentStatus = casesDomain.FitmentStatusStale
	service := newOrderBuilderService(world)
	_, err := service.PrepareOrderProposal(context.Background(), api.PrepareOrderProposalCommand{
		Actor:      testActor,
		TenantID:   testTenant,
		CaseID:     previewCaseID,
		WorkItemID: previewWorkID,
		Now:        previewNow,
	})
	if !errors.Is(err, domain.ErrOrderFitmentNotProven) {
		t.Fatalf("non-proven — отказ ErrOrderFitmentNotProven, получено: %v", err)
	}
}

// ---------------------------------------------------------------------------
// The dispatch ladder: the order composition revalidation fence
// ---------------------------------------------------------------------------

// fakeOrderVersionSource is the scriptable OrderVersionSource of the unit
// circuit (the digest answer, the optional error and the call counter).
type fakeOrderVersionSource struct {
	digest string
	err    error
	calls  int
}

func (s *fakeOrderVersionSource) OrderCommitmentDigest(_ context.Context, _ port.OrderVersionRequest) (string, error) {
	s.calls++
	return s.digest, s.err
}

// orderFenceDefs builds the guard-valid definition dictionary whose FIRST
// entry pins the order digest in its typed input schema (the structural
// address of the revalidation fence): the schema of the real entry over
// the permissive fixture conditions of the unit circuit.
func orderFenceDefs() []domain.ActionDefinition {
	defs := fixtureDefinitions()
	defs[0].TypedInputSchema = append([]domain.FieldSchema{
		{Name: "order_digest", Label: "Пин точной версии заказа", Type: domain.FieldText, Required: true, Description: "64-hex канонического состава."},
	}, defs[0].TypedInputSchema...)
	return defs
}

// newOrderFenceFixture builds the dispatch fixture over the order-pinning
// definition: the pinned preview carries the order digest typed input, the
// confirm/authorize flow is LIVING (the 3-5 machine), the fresh queued job
// is ready for the drain.
func newOrderFenceFixture(t *testing.T, pinnedDigest string) *dispatchFixture {
	t.Helper()
	auth := newAuthorizationFixtureWithDefs(t, orderFenceDefs())
	auth.preview.TypedInput["order_digest"] = pinnedDigest
	auth.preview.TypedInput["order_material"] = "позиция pos-1: Bosch / 341167866712 (оригинал)"
	auth.preview.Digest = domain.PreviewDigest(auth.previews.key, auth.preview.PreviewID, auth.preview.VersionNo, domain.PreviewCanonicalBytes(auth.preview))
	auth.ops.pinPreview(auth.preview)
	thread, version := auth.confirm(t)
	const key = "order-fence-unit-key-1"
	auth.authorize(t, thread, version, key)
	op, ok := auth.ops.operationOfKey(key)
	if !ok {
		t.Fatal("авторизация обязана оставить операцию в фейковом репозитории")
	}
	rawPayload, err := json.Marshal(auth.ops.outbox[op.OperationID])
	if err != nil {
		t.Fatalf("маршалинг SAFE-полезной нагрузки обязан проходить: %v", err)
	}
	repo := newFakeDispatchRepo()
	repo.jobs = []port.PendingDispatchJob{{
		EventID:        id.MustParse("01994f64-6c00-7000-8000-0000000000d1"),
		RawPayload:     rawPayload,
		Operation:      op,
		Ledger:         append([]domain.OperationLedgerEntry(nil), auth.ops.ledgers[op.OperationID]...),
		StateWord:      domain.OperationAuthorized,
		Classification: port.PendingClassFresh,
	}}
	repo.ledger[op.OperationID] = append([]domain.OperationLedgerEntry(nil), auth.ops.ledgers[op.OperationID]...)
	adapter := &fakeMutationAdapter{capability: auth.def.RequiredCapability.CapabilityID}
	auth.material.caseCalls = 0
	auth.material.factsCalls = 0
	return &dispatchFixture{auth: auth, thread: thread, version: version, key: key, op: op, repo: repo, adapter: adapter}
}

// TestDispatchOrderRevalidationMatchPasses — живой дайджест СОВПАДАЕТ с
// пином preview — забор проходит, операция диспетчерится (committed).
func TestDispatchOrderRevalidationMatchPasses(t *testing.T) {
	const pinned = "1111111111111111111111111111111111111111111111111111111111111111"
	f := newOrderFenceFixture(t, pinned)
	source := &fakeOrderVersionSource{digest: pinned}
	service := f.drainService(f.adapter, nil, f.auth.world.reader).WithOrderVersions(source)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("совпадающий дайджест обязан проходить забор: %v", err)
	}
	if report.Dispatched != 1 || report.Blocked != 0 {
		t.Fatalf("совпадение = диспетч: %+v", report)
	}
	if source.calls != 1 {
		t.Fatalf("источник живой версии читается ровно один раз: %d", source.calls)
	}
	if f.adapter.calls != 1 {
		t.Fatalf("адаптер вызван один раз: %d", f.adapter.calls)
	}
}

// TestDispatchOrderRevalidationDivergenceBlocks — живой дайджест
// РАСХОДИТСЯ с пином — dispatch_blocked точным словом состава, операция
// запаркована, lease/адаптер не тронуты.
func TestDispatchOrderRevalidationDivergenceBlocks(t *testing.T) {
	const pinned = "1111111111111111111111111111111111111111111111111111111111111111"
	f := newOrderFenceFixture(t, pinned)
	source := &fakeOrderVersionSource{digest: "2222222222222222222222222222222222222222222222222222222222222222"}
	service := f.drainService(f.adapter, nil, f.auth.world.reader).WithOrderVersions(source)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("расхождение — честный block, не ошибка: %v", err)
	}
	if report.Blocked != 1 || report.Dispatched != 0 {
		t.Fatalf("расхождение = запарковано: %+v", report)
	}
	if f.repo.leaseCalls != 0 || f.adapter.calls != 0 {
		t.Fatalf("забор до lease: lease=%d adapter=%d", f.repo.leaseCalls, f.adapter.calls)
	}
	blocked := false
	for _, entry := range f.repo.ledger[f.op.OperationID] {
		if entry.Kind == domain.LedgerEntryDispatchBlocked && strings.Contains(entry.Details, domain.OrderCompositionChangedWord) {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("ledger обязан нести dispatch_blocked с точным словом состава: %+v", f.repo.ledger[f.op.OperationID])
	}
}

// TestDispatchOrderRevalidationNilSourceFailsClosed — nil-порт —
// fail-closed: dispatch_blocked словом неподключённого источника.
func TestDispatchOrderRevalidationNilSourceFailsClosed(t *testing.T) {
	const pinned = "1111111111111111111111111111111111111111111111111111111111111111"
	f := newOrderFenceFixture(t, pinned)
	service := f.drainService(f.adapter, nil, f.auth.world.reader) // НИКАКОГО WithOrderVersions
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("nil-порт — честный block, не ошибка: %v", err)
	}
	if report.Blocked != 1 || report.Dispatched != 0 {
		t.Fatalf("nil-порт = запарковано: %+v", report)
	}
	blocked := false
	for _, entry := range f.repo.ledger[f.op.OperationID] {
		if entry.Kind == domain.LedgerEntryDispatchBlocked && strings.Contains(entry.Details, domain.OrderVersionSourceMissingWord) {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("ledger обязан нести слово fail-closed источника: %+v", f.repo.ledger[f.op.OperationID])
	}
	if f.repo.leaseCalls != 0 || f.adapter.calls != 0 {
		t.Fatalf("fail-closed до lease: lease=%d adapter=%d", f.repo.leaseCalls, f.adapter.calls)
	}
}

// TestDispatchOrderRevalidationRunsBeforeCaseLiveness — порядок шагов
// живым зондом: мёртвый кейс И расходящийся дайджест — побеждает слово
// СОСТАВА (забор revalidation стоит ПОСЛЕ confirmation-fence ДО
// case/workitem-alive).
func TestDispatchOrderRevalidationRunsBeforeCaseLiveness(t *testing.T) {
	const pinned = "1111111111111111111111111111111111111111111111111111111111111111"
	f := newOrderFenceFixture(t, pinned)
	// The case dies AFTER the authorization (the fixture build): the
	// work items vanish — the liveness fence would block with its own
	// word IF the order fence had not spoken first.
	f.auth.world.caseView.WorkItems = nil
	source := &fakeOrderVersionSource{digest: "2222222222222222222222222222222222222222222222222222222222222222"}
	service := f.drainService(f.adapter, nil, f.auth.world.reader).WithOrderVersions(source)
	report, err := service.DispatchQueuedOperations(context.Background(), f.command())
	if err != nil {
		t.Fatalf("зонд порядка — честный block: %v", err)
	}
	if report.Blocked != 1 {
		t.Fatalf("запарковано один раз: %+v", report)
	}
	blocked := false
	for _, entry := range f.repo.ledger[f.op.OperationID] {
		if entry.Kind == domain.LedgerEntryDispatchBlocked && strings.Contains(entry.Details, domain.OrderCompositionChangedWord) {
			blocked = true
		}
		if entry.Kind == domain.LedgerEntryDispatchBlocked && strings.Contains(entry.Details, "кейс закрыт") {
			t.Fatal("слово закрытого кейса НЕ обязано звучать первым — забор состава идёт раньше")
		}
	}
	if !blocked {
		t.Fatalf("первым говорит слово состава: %+v", f.repo.ledger[f.op.OperationID])
	}
}

// ---------------------------------------------------------------------------
// The receipt render: the derived words of the committed order receipt
// ---------------------------------------------------------------------------

// TestOrderReceiptRenderSpeaksTheDerivedWords — квитанция committed ДЕЙСТВИЯ
// ЗАКАЗА несёт производные слова чтения: состояние
// pending_manager_verification, клиентские слова, слово назначения
// менеджера и 1С-unknown. Квитанции failed/unknown и квитанции ДРУГИХ
// действий этих слов НЕ несут (честные слова 3-6 без изменений).
func TestOrderReceiptRenderSpeaksTheDerivedWords(t *testing.T) {
	committedOrder := port.ActionReceiptRow{
		ReceiptVersionNo: 1,
		Outcome:          domain.ReceiptOutcomeCommitted,
		References:       port.ReceiptReferences{DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion},
		Details:          "квитанция провайдера",
	}
	views := actionReceiptViews([]port.ActionReceiptRow{committedOrder})
	if len(views) != 1 {
		t.Fatalf("одна квитанция, получено %d", len(views))
	}
	view := views[0]
	if view.OrderState != domain.OrderReceiptStatePendingManagerVerification {
		t.Fatalf("состояние заказа — pending_manager_verification, получено %q", view.OrderState)
	}
	if view.OrderStateWord != domain.OrderClientCreatedWord {
		t.Fatalf("клиентское слово состояния: %q", view.OrderStateWord)
	}
	if view.OrderBoundaryWord != domain.OrderClientBoundaryWord {
		t.Fatalf("граничное слово клиента: %q", view.OrderBoundaryWord)
	}
	if view.OrderOneCWord != domain.OrderOneCUnknownWord {
		t.Fatalf("1С-слово unknown: %q", view.OrderOneCWord)
	}
	if view.OrderManagerWord != domain.OrderManagerAssignmentWord {
		t.Fatalf("слово назначения менеджера: %q", view.OrderManagerWord)
	}

	// failed/unknown order receipts carry NO derived words (the honest
	// 3-6 words stay — the render never dresses a failure up).
	for _, outcome := range []string{domain.ReceiptOutcomeFailed, domain.ReceiptOutcomeUnknown} {
		row := committedOrder
		row.Outcome = outcome
		plain := actionReceiptViews([]port.ActionReceiptRow{row})[0]
		if plain.OrderState != "" || plain.OrderStateWord != "" || plain.OrderOneCWord != "" {
			t.Fatalf("квитанция %q заказа НЕ несёт производных слов заказа: %+v", outcome, plain)
		}
	}

	// committed receipts of OTHER actions carry no order words either.
	other := committedOrder
	other.References.DefinitionID = "quote.send-price-quote"
	plainOther := actionReceiptViews([]port.ActionReceiptRow{other})[0]
	if plainOther.OrderState != "" || plainOther.OrderManagerWord != "" {
		t.Fatalf("квитанция другого действия НЕ несёт слов заказа: %+v", plainOther)
	}
}
