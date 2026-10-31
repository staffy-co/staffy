package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/action/port"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.11 step-02: the pilot outcome application contour (the read-only
// case outcome derivation over the LIVE material, the honest degradation
// of the unwired contour, the review gate and the period report)
// ---------------------------------------------------------------------------

// The fixture case identities of the pilot world: the full-journey case
// (the committed order with the approved verification and the sent
// supplier fact), the fresh case (the inquiry only) and the outside
// period row of the report filter probe.
var (
	pilotCaseFull   = id.MustParse("01994f64-6c00-7000-8000-0000000000d1")
	pilotCaseFresh  = id.MustParse("01994f64-6c00-7000-8000-0000000000d2")
	pilotCaseOther  = id.MustParse("01994f64-6c00-7000-8000-0000000000d3")
	pilotOperation  = id.MustParse("01994f64-6c00-7000-8000-0000000000d4")
	pilotRecoveryID = id.MustParse("01994f64-6c00-7000-8000-0000000000d5")
)

// pilotCaseSourceFake is the in-memory PilotCaseSource of the pilot
// tests: the listing view of the report sample and the per-case views of
// the assembly (the fakeCaseMaterial precedent with the listing read
// added; a map miss is the honest foreign-case 404).
type pilotCaseSourceFake struct {
	listing casesAPI.CasesView
	views   map[id.UUID]casesAPI.CaseView
}

func (f pilotCaseSourceFake) CasesView(_ context.Context, _ identityDomain.Principal, _ id.UUID) (casesAPI.CasesView, error) {
	return f.listing, nil
}

func (f pilotCaseSourceFake) CaseView(_ context.Context, _ identityDomain.Principal, _, caseID id.UUID) (casesAPI.CaseView, error) {
	if view, known := f.views[caseID]; known {
		return view, nil
	}
	return casesAPI.CaseView{}, domain.ErrNotFound
}

func (f pilotCaseSourceFake) FactsView(_ context.Context, _ identityDomain.Principal, _, caseID id.UUID) (casesAPI.FactsView, error) {
	return casesAPI.FactsView{TenantID: testTenant, CaseID: caseID}, nil
}

// pilotFullCaseView builds the full-journey case view: the accepted
// inbound channel event (the attribution material), the confirmed
// vehicle attribute with the satisfied completeness, the confirmed part
// request attribute, the proven fitment position, one work item and one
// handoff request.
func pilotFullCaseView() casesAPI.CaseView {
	confirmedAt := previewNow.Add(time.Hour)
	return casesAPI.CaseView{
		TenantID: testTenant,
		Case:     casesDomain.Case{TenantID: testTenant, CaseID: pilotCaseFull, CaseNo: 42, EmployeeID: previewEmployee, CreatedAt: previewNow},
		Timeline: []casesDomain.TimelineEvent{{
			Channel: "chat", Direction: casesDomain.DirectionInbound,
			Provider: "jivo", ProviderEventID: "evt-9", State: casesDomain.EventStateAccepted,
			ProviderTimestamp: previewNow, ReceivedAt: previewNow,
		}},
		WorkItems: []casesDomain.WorkItemCurrent{{WorkItemID: previewWorkID, CaseID: pilotCaseFull, Version: 1}},
		Vehicle: casesAPI.VehiclePanelView{
			SchemaDefined: true,
			Profile: casesDomain.VehicleProfileView{
				Attributes: []casesDomain.VehicleAttributeView{{
					MaterialKey: "vin", Label: "VIN", Kind: "vin", RawValue: "XTA21099",
					FactVersion: 1, Status: "confirmed",
					Confirmation: &casesDomain.VehicleConfirmationView{ConfirmerType: "manager", VersionNo: 1, At: confirmedAt},
				}},
				Completeness: casesDomain.VehicleCompleteness{SatisfiedBy: []string{"vin"}},
			},
		},
		PartRequest: casesAPI.PartPanelView{
			SchemaDefined: true,
			Part: casesDomain.PartRequestView{
				Mode: "structured",
				Attributes: []casesDomain.PartAttributeView{{
					MaterialKey: "oem_number", Label: "Номер детали", Role: "oem_number", Kind: "text",
					RawValue: "2108-8403010", FactVersion: 1, Status: "confirmed",
					Confirmation: &casesDomain.PartConfirmationView{ConfirmerType: "manager", VersionNo: 1, At: confirmedAt},
				}},
			},
		},
		Fitment: casesAPI.FitmentPanelView{
			SchemaDefined: true,
			Fitment: casesDomain.FitmentView{
				Positions: []casesDomain.FitmentPositionView{{
					PositionKey: "front-left", Status: casesDomain.FitmentStatusProven,
					StatusWord: "применимость доказана",
					Evidences:  []casesDomain.FitmentEvidenceView{{EvidenceNo: 1, ReceivedAt: previewNow.Add(2 * time.Hour), Found: true}},
				}},
			},
		},
		Handoff: casesAPI.HandoffView{RequestsCount: 1},
	}
}

// pilotFreshCaseView builds the fresh case view: the case exists (the
// inquiry stage), nothing else — the honest not_reached ladder.
func pilotFreshCaseView() casesAPI.CaseView {
	return casesAPI.CaseView{
		TenantID: testTenant,
		Case:     casesDomain.Case{TenantID: testTenant, CaseID: pilotCaseFresh, CaseNo: 43, EmployeeID: previewEmployee, CreatedAt: previewNow},
	}
}

// pilotFixture is the full world of the pilot tests: the pilot case
// source with the per-case views, the order machine fakes (the committed
// receipt with the external ABCP Order ID, the approved verification
// decision, the sent supplier fact, the correction row and the recovery
// case row) and the wired service.
type pilotFixture struct {
	cases         pilotCaseSourceFake
	verifications *fakeVerificationRepo
	facts         *fakeOrderFactRepo
	service       *Service
}

// newPilotFixture wires the service over the full-journey world (the
// unit composition root of the pilot tests — the statusFixture
// precedent over the SHARED operation/dispatch/verification/correction/
// fact/recovery fakes).
func newPilotFixture(t *testing.T, identity scopeIdentity, listing casesAPI.CasesView, views map[id.UUID]casesAPI.CaseView) *pilotFixture {
	t.Helper()
	ops := newFakeOperationRepo(newFakeConfirmationRepo())
	operation := domain.Operation{
		TenantID:     testTenant,
		OperationID:  pilotOperation,
		DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
		CaseID: pilotCaseFull, WorkItemID: previewWorkID,
		OwnerKind: "human", OwnerRef: verificationOwner, OwnerEpoch: 1,
		Partition: domain.PartitionHumanOnly, CreatedAt: previewNow,
	}
	ops.operations = []domain.Operation{operation}
	dispatch := newFakeDispatchRepo()
	dispatch.receipts[pilotOperation] = []port.RecordDispatchOutcomeParams{{
		ReceiptVersionNo: 1, ReceiptWord: domain.ReceiptOutcomeCommitted,
		ReceiptReferences: port.ReceiptReferences{ExternalObjectID: "ABCP-42", ExternalObjectVersion: "v1"},
		Now:               previewNow.Add(30 * time.Minute),
	}}
	verifications := newFakeVerificationRepo()
	verifications.tasks = []domain.OrderVerification{{
		TenantID: testTenant, CaseID: pilotCaseFull, WorkItemID: previewWorkID,
		VerificationNo: 1, OperationID: pilotOperation, ReceiptVersionNo: 1,
		ExternalOrderID: "ABCP-42", CreatorID: verificationOwner, CreatedAt: previewNow,
	}}
	verifications.events[1] = []domain.OrderVerificationEvent{{
		TenantID: testTenant, CaseID: pilotCaseFull, VerificationNo: 1, EventNo: 1,
		Kind: domain.OrderVerificationEventApproved, OrderDigest: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		ReceiptVersionNo: 1, ExternalOrderID: "ABCP-42",
		ActorID: verificationManager, CreatedAt: previewNow.Add(3 * time.Hour),
	}}
	corrections := newFakeCorrectionRepo()
	corrections.corrections = []domain.OrderCorrection{{
		TenantID: testTenant, CaseID: pilotCaseFull, WorkItemID: previewWorkID,
		CorrectionNo: 1, OperationID: pilotOperation, VerificationNo: 1, ReceiptVersionNo: 1,
		ExternalOrderID: "ABCP-42", CreatorID: verificationManager, CreatedAt: previewNow.Add(2 * time.Hour),
	}}
	facts := newFakeOrderFactRepo()
	facts.facts = []domain.OrderSupplierFact{{
		TenantID: testTenant, CaseID: pilotCaseFull, WorkItemID: previewWorkID, FactNo: 1,
		OperationID: pilotOperation, Kind: domain.OrderSupplierFactSent, SupplierOrderID: "SUP-7",
		HappenedAt: previewNow.Add(4 * time.Hour), ActorID: verificationManager, RecordedAt: previewNow.Add(4*time.Hour + time.Minute),
	}}
	recoveries := newFakeRecoveryRepo(port.RecoveryOperationFacts{})
	recoveries.rows[pilotOperation] = &port.RecoveryCaseRow{
		RecoveryCaseID: pilotRecoveryID, RecoveryCaseNo: 1, OperationID: pilotOperation,
	}
	casesSource := pilotCaseSourceFake{listing: listing, views: views}
	service := New(identity, acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
		WithPreviews(newFakePreviewRepo(), nil, fakeCaseMaterial{}, verificationMembers{},
			fakeEntitlement{status: "active"}, fakeReadiness{}, orderConnectors{}, fakeRouteFacts{}, &fakeIDs{}).
		WithAuthorization(newFakeConfirmationRepo(), ops).
		WithDispatch(dispatch, nil).
		WithOrderVerifications(verifications).
		WithOrderCorrections(corrections).
		WithOrderSupplierFacts(facts).
		WithRecovery(recoveries).
		WithPilotOutcome(casesSource)
	return &pilotFixture{cases: casesSource, verifications: verifications, facts: facts, service: service}
}

// pilotReviewIdentity is the review-member identity of the happy
// reads (the scopeIdentity fake — the membership plus the review
// scope).
func pilotReviewIdentity() scopeIdentity {
	return scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}}
}

// pilotStageOf finds one stage of the outcome BY NAME (the dictionary
// word is the test anchor — the state words ride the assertion words).
func pilotStageOf(t *testing.T, outcome domain.PilotCaseOutcome, stage string) domain.PilotStageEvidence {
	t.Helper()
	for _, row := range outcome.Stages {
		if row.Stage == stage {
			return row
		}
	}
	t.Fatalf("стадия %s отсутствует в исходе", stage)
	return domain.PilotStageEvidence{}
}

// TestReadPilotCaseOutcomeDegradation pins the honest refusal of the
// unwired contour: a fresh service WITHOUT the pilot outcome wiring
// answers the spoken word — never a nil panic, never an invented
// material.
func TestReadPilotCaseOutcomeDegradation(t *testing.T) {
	unwired := New(pilotReviewIdentity(), acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs())
	_, err := unwired.ReadPilotCaseOutcome(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, pilotCaseFull, previewNow)
	if err == nil || !strings.Contains(err.Error(), "контур исхода обращения не подключён") {
		t.Fatalf("неподключённый контур исхода: %v", err)
	}
	_, err = unwired.ReadPilotReport(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, "", previewNow)
	if err == nil || !strings.Contains(err.Error(), "контур исхода обращения не подключён") {
		t.Fatalf("неподключённый контур отчёта: %v", err)
	}
}

// TestReadPilotCaseOutcomeScopeRefusal pins the read refusals of the
// gate: the 404 of a non-member (the honest tenant blindness) and the
// 403 scope word of the member WITHOUT the review scope (the words
// BEFORE any outcome material — the order status precedent).
func TestReadPilotCaseOutcomeScopeRefusal(t *testing.T) {
	// The non-member: the honest 404 (the fully-wired service over the
	// non-member identity — the membership flag of the scopeIdentity
	// fake; the ports are wired so the gate itself answers).
	nonMember := newPilotFixture(t, scopeIdentity{member: false}, casesAPI.CasesView{},
		map[id.UUID]casesAPI.CaseView{pilotCaseFull: pilotFullCaseView()})
	_, err := nonMember.service.ReadPilotCaseOutcome(context.Background(),
		identityDomain.Principal{ID: verificationNoScope}, testTenant, pilotCaseFull, previewNow)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник: %v", err)
	}
	_, err = nonMember.service.ReadPilotReport(context.Background(),
		identityDomain.Principal{ID: verificationNoScope}, testTenant, "", previewNow)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник (отчёт): %v", err)
	}
	// The member without the review scope: the 403 with the scope word
	// (the fully-wired no-scope service — the 4-9 precedent).
	noScope := newPilotFixture(t, scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{}},
		casesAPI.CasesView{}, map[id.UUID]casesAPI.CaseView{pilotCaseFull: pilotFullCaseView()})
	_, err = noScope.service.ReadPilotCaseOutcome(context.Background(),
		identityDomain.Principal{ID: verificationNoScope}, testTenant, pilotCaseFull, previewNow)
	if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "полномочия review") {
		t.Fatalf("актёр без review: %v", err)
	}
	_, err = noScope.service.ReadPilotReport(context.Background(),
		identityDomain.Principal{ID: verificationNoScope}, testTenant, "", previewNow)
	if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "полномочия review") {
		t.Fatalf("актёр без review (отчёт): %v", err)
	}
}

// TestReadPilotCaseOutcomeDerivation pins the live derivation over the
// full-journey world: the attribution of the first accepted inbound
// channel event BY NAME, the six proven stages with their evidence
// sources, the four terminal unknown_pending stages (the honest pins of
// the out-of-contour owning sources) and the recorded counters.
func TestReadPilotCaseOutcomeDerivation(t *testing.T) {
	f := newPilotFixture(t, pilotReviewIdentity(), casesAPI.CasesView{}, map[id.UUID]casesAPI.CaseView{pilotCaseFull: pilotFullCaseView()})
	outcome, err := f.service.ReadPilotCaseOutcome(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, pilotCaseFull, previewNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("чтение исхода обязано пройти: %v", err)
	}
	if !outcome.CaseFound || outcome.CaseNo != 42 {
		t.Fatalf("исход: found=%v case_no=%d", outcome.CaseFound, outcome.CaseNo)
	}
	if !outcome.Attribution.Found || outcome.Attribution.Provider != "jivo" ||
		outcome.Attribution.Channel != "chat" || outcome.Attribution.ProviderEventID != "evt-9" {
		t.Fatalf("атрибуция: %+v", outcome.Attribution)
	}
	if !strings.Contains(outcome.Attribution.Word, "jivo") || !strings.Contains(outcome.Attribution.Word, "evt-9") {
		t.Fatalf("слово атрибуции: %s", outcome.Attribution.Word)
	}
	inquiry := pilotStageOf(t, outcome, domain.PilotStageInquiryAccepted)
	if inquiry.State != domain.PilotStageStateProven || !inquiry.Time.Equal(previewNow) {
		t.Fatalf("стадия обращения: %+v", inquiry)
	}
	vehicle := pilotStageOf(t, outcome, domain.PilotStageVehicleProfileConfirmed)
	if vehicle.State != domain.PilotStageStateProven || !strings.Contains(vehicle.Evidence, "vin") {
		t.Fatalf("стадия профиля: %+v", vehicle)
	}
	fitment := pilotStageOf(t, outcome, domain.PilotStageApplicabilityProven)
	if fitment.State != domain.PilotStageStateProven || !strings.Contains(fitment.Evidence, "front-left") {
		t.Fatalf("стадия применимости: %+v", fitment)
	}
	created := pilotStageOf(t, outcome, domain.PilotStageClientOrderCreated)
	if created.State != domain.PilotStageStateProven || !strings.Contains(created.Evidence, "ABCP-42") {
		t.Fatalf("стадия заказа: %+v", created)
	}
	verified := pilotStageOf(t, outcome, domain.PilotStageManagerVerified)
	if verified.State != domain.PilotStageStateProven {
		t.Fatalf("стадия проверки: %+v", verified)
	}
	sent := pilotStageOf(t, outcome, domain.PilotStageSupplierSent)
	if sent.State != domain.PilotStageStateProven || !strings.Contains(sent.Evidence, "SUP-7") {
		t.Fatalf("стадия отправки: %+v", sent)
	}
	for _, stage := range domain.PilotTerminalStages {
		row := pilotStageOf(t, outcome, stage)
		if row.State != domain.PilotStageStateUnknownPending || row.PinWord == "" {
			t.Fatalf("терминальная стадия %s: %+v", stage, row)
		}
	}
	if outcome.HandoffCount != 1 || outcome.CorrectionsCount != 1 || outcome.RecoveryCount != 1 {
		t.Fatalf("счётчики: handoff=%d corrections=%d recovery=%d", outcome.HandoffCount, outcome.CorrectionsCount, outcome.RecoveryCount)
	}
}

// TestReadPilotCaseOutcomeFreshCase pins the honest fresh-case ladder:
// the inquiry stage proven by the case itself, every later stage
// not_reached (the honest absence words) and the four terminal
// unknown_pending stages (never zero, never excluded).
func TestReadPilotCaseOutcomeFreshCase(t *testing.T) {
	f := newPilotFixture(t, pilotReviewIdentity(), casesAPI.CasesView{}, map[id.UUID]casesAPI.CaseView{pilotCaseFresh: pilotFreshCaseView()})
	outcome, err := f.service.ReadPilotCaseOutcome(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, pilotCaseFresh, previewNow)
	if err != nil {
		t.Fatalf("чтение исхода свежего кейса обязано пройти: %v", err)
	}
	if !outcome.CaseFound || outcome.Attribution.Found {
		t.Fatalf("свежий кейс: found=%v attribution=%+v", outcome.CaseFound, outcome.Attribution)
	}
	if !strings.Contains(outcome.Attribution.Word, "принятых канальных событий нет") {
		t.Fatalf("слово атрибуции свежего кейса: %s", outcome.Attribution.Word)
	}
	for _, stage := range []string{
		domain.PilotStageVehicleProfileConfirmed, domain.PilotStageApplicabilityProven,
		domain.PilotStageClientOrderCreated, domain.PilotStageManagerVerified, domain.PilotStageSupplierSent,
	} {
		if row := pilotStageOf(t, outcome, stage); row.State != domain.PilotStageStateNotReached {
			t.Fatalf("стадия %s свежего кейса: %+v", stage, row)
		}
	}
}

// TestReadPilotReportDerivation pins the aggregate report of the period:
// the sample volume, the stage counts over the per-case derivations, the
// five conversions of the PROVEN neighbours, the honest unknown share
// (the terminal outcomes live outside the contour) and the period word.
func TestReadPilotReportDerivation(t *testing.T) {
	listing := casesAPI.CasesView{TenantID: testTenant, CanView: true, ViewScope: casesAPI.CaseScopeView, Cases: []casesDomain.CaseSummary{
		{CaseID: pilotCaseFull, CaseNo: 42, CreatedAt: previewNow},
		{CaseID: pilotCaseFresh, CaseNo: 43, CreatedAt: previewNow},
	}}
	f := newPilotFixture(t, pilotReviewIdentity(), listing, map[id.UUID]casesAPI.CaseView{
		pilotCaseFull:  pilotFullCaseView(),
		pilotCaseFresh: pilotFreshCaseView(),
	})
	report, err := f.service.ReadPilotReport(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, "2026-10", previewNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("чтение отчёта обязано пройти: %v", err)
	}
	if report.CaseVolume != 2 || report.SampleSize != 2 {
		t.Fatalf("объём выборки: %d/%d", report.CaseVolume, report.SampleSize)
	}
	if report.StageCounts[domain.PilotStageInquiryAccepted] != 2 {
		t.Fatalf("обращений принято: %d", report.StageCounts[domain.PilotStageInquiryAccepted])
	}
	if report.StageCounts[domain.PilotStageClientOrderCreated] != 1 ||
		report.StageCounts[domain.PilotStageManagerVerified] != 1 ||
		report.StageCounts[domain.PilotStageSupplierSent] != 1 {
		t.Fatalf("счётчики стадий полного пути: %+v", report.StageCounts)
	}
	if len(report.Conversions) != 5 {
		t.Fatalf("конверсий %d, а не 5", len(report.Conversions))
	}
	if report.UnknownShare.Numerator != 2 || report.UnknownShare.Denominator != 2 {
		t.Fatalf("доля неизвестных: %d/%d", report.UnknownShare.Numerator, report.UnknownShare.Denominator)
	}
	if report.HandoffCount != 1 || report.CorrectionsCount != 1 || report.RecoveryCount != 1 {
		t.Fatalf("счётчики отчёта: handoff=%d corrections=%d recovery=%d", report.HandoffCount, report.CorrectionsCount, report.RecoveryCount)
	}
	if report.PeriodWord != pilotOutcomePeriodWordPrefix+"2026-10" {
		t.Fatalf("слово периода: %s", report.PeriodWord)
	}
	// The whole-period word of the absent period.
	report, err = f.service.ReadPilotReport(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, "", previewNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("чтение отчёта всего периода обязано пройти: %v", err)
	}
	if report.PeriodWord != pilotOutcomeWholePeriodWord {
		t.Fatalf("слово всего периода: %s", report.PeriodWord)
	}
}

// TestReadPilotReportPeriodFilter pins the honest period rule: ONLY the
// cases created in the month of a WELL-FORMED YYYY-MM period enter the
// sample; the malformed period word passes through with NO filtering
// (the word rides the report, nothing is guessed).
func TestReadPilotReportPeriodFilter(t *testing.T) {
	listing := casesAPI.CasesView{TenantID: testTenant, CanView: true, ViewScope: casesAPI.CaseScopeView, Cases: []casesDomain.CaseSummary{
		{CaseID: pilotCaseFull, CaseNo: 42, CreatedAt: previewNow},
		{CaseID: pilotCaseFresh, CaseNo: 43, CreatedAt: previewNow},
		{CaseID: pilotCaseOther, CaseNo: 44, CreatedAt: previewNow.AddDate(0, -1, 0)},
	}}
	f := newPilotFixture(t, pilotReviewIdentity(), listing, map[id.UUID]casesAPI.CaseView{
		pilotCaseFull:  pilotFullCaseView(),
		pilotCaseFresh: pilotFreshCaseView(),
		pilotCaseOther: pilotFreshCaseView(),
	})
	report, err := f.service.ReadPilotReport(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, "2026-10", previewNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("чтение отчёта периода обязано пройти: %v", err)
	}
	if report.CaseVolume != 2 {
		t.Fatalf("объём периода 2026-10: %d, а не 2 (внешняя строка обязана отфильтроваться)", report.CaseVolume)
	}
	// The malformed period word: NO filtering — the whole observation
	// period honestly answers with the word riding the report.
	report, err = f.service.ReadPilotReport(context.Background(),
		identityDomain.Principal{ID: verificationOwner}, testTenant, "октопус", previewNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("чтение отчёта с некорректным периодом обязано пройти: %v", err)
	}
	if report.CaseVolume != 3 {
		t.Fatalf("объём некорректного периода: %d, а не 3 (фильтрации нет)", report.CaseVolume)
	}
	if report.PeriodWord != pilotOutcomePeriodWordPrefix+"октопус" {
		t.Fatalf("слово некорректного периода: %s", report.PeriodWord)
	}
}
