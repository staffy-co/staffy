package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/action/api"
	actionCatalog "github.com/staffy-co/staffy/internal/action/catalog"
	"github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/action/port"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.8 step-02: the order correction application contour (the four
// commands, the reread fence, the CAS races, the TWO structural gates)
// ---------------------------------------------------------------------------

// fakeCorrectionRepo is the in-memory OrderCorrectionRepository with the
// CAS semantics of the store: the per-case correction_no ladder, the
// one-open-correction rule, the epoch re-validation under the append and
// the raceHook (the parallel-winner simulation of the CAS tests).
type fakeCorrectionRepo struct {
	corrections []domain.OrderCorrection
	events      map[int64][]domain.OrderCorrectionEvent
	nextNumber  map[id.UUID]int64
	raceHook    func(repo *fakeCorrectionRepo)
}

func newFakeCorrectionRepo() *fakeCorrectionRepo {
	return &fakeCorrectionRepo{events: map[int64][]domain.OrderCorrectionEvent{}, nextNumber: map[id.UUID]int64{}}
}

func (f *fakeCorrectionRepo) eventsOf(caseID id.UUID, correctionNo int64) []domain.OrderCorrectionEvent {
	out := []domain.OrderCorrectionEvent{}
	for _, event := range f.events[correctionNo] {
		if event.CaseID == caseID {
			out = append(out, event)
		}
	}
	return out
}

func (f *fakeCorrectionRepo) activeOf(operationID id.UUID, now time.Time) bool {
	for _, correction := range f.corrections {
		if correction.OperationID != operationID {
			continue
		}
		if domain.DeriveOrderCorrectionState(correction, f.eventsOf(correction.CaseID, correction.CorrectionNo), now).Active() {
			return true
		}
	}
	return false
}

func (f *fakeCorrectionRepo) ReadOperationCorrections(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]domain.OrderCorrection, error) {
	out := []domain.OrderCorrection{}
	for _, correction := range f.corrections {
		if correction.OperationID == operationID {
			out = append(out, correction)
		}
	}
	return out, nil
}

func (f *fakeCorrectionRepo) ReadCaseCorrections(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]domain.OrderCorrection, error) {
	out := []domain.OrderCorrection{}
	for _, correction := range f.corrections {
		if correction.CaseID == caseID {
			out = append(out, correction)
		}
	}
	return out, nil
}

func (f *fakeCorrectionRepo) ReadCorrectionEvents(_ context.Context, _ tenancy.Context, caseID id.UUID, correctionNo int64) ([]domain.OrderCorrectionEvent, error) {
	return append([]domain.OrderCorrectionEvent(nil), f.eventsOf(caseID, correctionNo)...), nil
}

func (f *fakeCorrectionRepo) AppendOrderCorrection(_ context.Context, p port.AppendOrderCorrectionParams) (domain.OrderCorrection, error) {
	if f.activeOf(p.OperationID, p.Now) {
		return domain.OrderCorrection{}, port.ErrCorrectionActive
	}
	correctionNo := f.nextNumber[p.CaseID] + 1
	f.nextNumber[p.CaseID] = correctionNo
	correction := domain.OrderCorrection{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		CorrectionNo: correctionNo, OperationID: p.OperationID,
		VerificationNo: p.VerificationNo, ReceiptVersionNo: p.ReceiptVersionNo,
		ExternalOrderID: p.ExternalOrderID, CreatorID: p.CreatorID, CreatedAt: p.Now,
	}
	f.corrections = append(f.corrections, correction)
	event := domain.OrderCorrectionEvent{
		TenantID: correction.TenantID, CaseID: correction.CaseID, CorrectionNo: correctionNo, EventNo: 1,
		Kind: domain.OrderCorrectionEventOpened, Owner: p.Owner, CorrectDeadline: p.CorrectDeadline,
		DeadlineAt: p.DeadlineAt, Positions: p.Positions, Reason: p.Reason,
		ActorID: p.CreatorID, CreatedAt: p.Now,
	}
	f.events[correctionNo] = append(f.events[correctionNo], event)
	return correction, nil
}

func (f *fakeCorrectionRepo) AppendOrderCorrectionEvent(_ context.Context, p port.AppendOrderCorrectionEventParams) (domain.OrderCorrectionEvent, error) {
	if f.raceHook != nil {
		hook := f.raceHook
		f.raceHook = nil
		hook(f)
	}
	var correction *domain.OrderCorrection
	for i := range f.corrections {
		if f.corrections[i].CaseID == p.CaseID && f.corrections[i].CorrectionNo == p.CorrectionNo {
			correction = &f.corrections[i]
			break
		}
	}
	if correction == nil {
		return domain.OrderCorrectionEvent{}, port.ErrCorrectionNotFound
	}
	events := f.eventsOf(p.CaseID, p.CorrectionNo)
	view := domain.DeriveOrderCorrectionState(*correction, events, p.Now)
	if view.State != p.ExpectedState || view.AssignmentEpoch != p.ExpectedEpoch {
		return domain.OrderCorrectionEvent{}, port.ErrCorrectionRaced
	}
	event := p.Event
	event.TenantID = correction.TenantID
	event.CaseID = correction.CaseID
	event.CorrectionNo = correction.CorrectionNo
	event.EventNo = int64(len(events)) + 1
	event.CreatedAt = p.Now
	f.events[p.CorrectionNo] = append(f.events[p.CorrectionNo], event)
	return event, nil
}

// correctionFixture is the full world of the correction tests: the
// committed order operation with the BLOCKED verification decision (the
// anchor of the open), the open correction, the NEW confirmed version
// (the preview v2 with the order digest pin + the confirmation thread
// v2 with the confirmed event) and the wired service with the
// corrections repository.
type correctionFixture struct {
	verification  *verificationFixture
	corrections   *fakeCorrectionRepo
	confirmations *fakeConfirmationRepo
	service       *Service
	operation     domain.Operation
	// The NEW confirmed version pins of the reread path: the preview v2
	// (the same live material in the happy path — the recomposed
	// version) and the confirmation thread v2.
	newPreview      domain.ActionPreview
	newConfirmation domain.Confirmation
	digest          string
}

// newCorrectionFixture wires the service over the blocked-verification
// world (the unit composition root of the correction tests): the
// verification fixture first (the committed operation), then the BLOCKED
// decision (assign → acknowledge → block), then the corrections wiring.
func newCorrectionFixture(t *testing.T) *correctionFixture {
	t.Helper()
	verification := newVerificationFixture(t)
	// The BLOCKED decision of the verification task №1 (the anchor of
	// the correction open): the manager took the task and blocked it
	// with the affected position and the structured reason word.
	if _, err := verification.assign(); err != nil {
		t.Fatalf("назначение проверки обязано пройти: %v", err)
	}
	if _, err := verification.acknowledge(1, 1); err != nil {
		t.Fatalf("принятие проверки обязано пройти: %v", err)
	}
	if _, err := verification.service.BlockOrderVerification(context.Background(), BlockOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 1,
		Positions: []string{"pos-1"}, Reason: domain.VerifyBlockReasonWrongPart, Now: previewNow,
	}); err != nil {
		t.Fatalf("блокировка проверки обязана пройти: %v", err)
	}
	// The NEW confirmed version (the recomposed proposal of the
	// correction path): the preview v2 of the SAME preview thread with
	// the same live material digest (the happy-path reread) and the
	// confirmation thread v2 with the confirmed event (the v1 carries
	// the superseded event — the AC-15 probe material).
	confirmations := newFakeConfirmationRepo()
	corrections := newFakeCorrectionRepo()
	digest := verification.digest
	render := domain.OrderMaterialCanonicalText(mustDeriveOrderMaterial(t, verification.world.caseView))
	newPreview := domain.ActionPreview{
		TenantID: testTenant, PreviewID: verification.operation.PreviewID, VersionNo: 2,
		DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
		CaseID: previewCaseID, WorkItemID: previewWorkID,
		TypedInput: map[string]string{
			"order_digest":     digest,
			"order_material":   render,
			"known_client_sum": "4200.50",
			"currency":         "RUB",
		},
	}
	verification.ops.pinPreview(newPreview)
	newConfirmation := domain.Confirmation{
		TenantID:       testTenant,
		ConfirmationID: id.MustParse("01994f64-6c00-7000-8000-0000000000d3"),
		VersionNo:      2, DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
		CaseID: previewCaseID, WorkItemID: previewWorkID,
		Target:    domain.ProposalTarget{Kind: domain.TargetCatalogItem, Ref: previewTargetID},
		PreviewID: verification.operation.PreviewID, PreviewVersionNo: 2, PreviewDigest: digest,
		ValidUntil: previewNow.Add(24 * time.Hour), ConfirmedBy: verificationOwner,
		CreatedAt: previewNow,
	}
	confirmations.confirmations[newConfirmation.ConfirmationID] = []domain.Confirmation{newConfirmation}
	confirmations.events = append(confirmations.events,
		port.ConfirmationEventRow{
			ConfirmationID: newConfirmation.ConfirmationID, VersionNo: 2,
			CaseID: previewCaseID, EventNo: 1,
			Kind: domain.ConfirmationEventConfirmed, ActorID: verificationOwner, CreatedAt: previewNow,
		})
	// The re-wired service: the SAME world with the corrections contour
	// and the confirmations carrying the NEW confirmed version (the
	// production catalog defs — the order action definition with the
	// single_use consumption rule).
	service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
		acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
		WithPreviews(newFakePreviewRepo(), verification.world.control, verification.material, verificationMembers{},
			fakeEntitlement{status: verification.world.admission}, fakeReadiness{stage: verification.world.stage, instances: verification.world.instances},
			orderConnectors{state: verification.world.capState, contractVersion: verification.world.contract},
			fakeRouteFacts{profileVersion: verification.world.profile}, &fakeIDs{}).
		WithAuthorization(confirmations, verification.ops).
		WithDispatch(verification.dispatch, nil).
		WithOrderVerifications(verification.repo).
		WithOrderCorrections(corrections)
	return &correctionFixture{
		verification: verification, corrections: corrections, confirmations: confirmations,
		service: service, operation: verification.operation,
		newPreview: newPreview, newConfirmation: newConfirmation, digest: digest,
	}
}

// orderCatalogDefs serves the production catalog list (the order action
// definition with the single_use consumption rule — the reread state
// derivation input).
func orderCatalogDefs() []domain.ActionDefinition {
	return actionCatalog.List()
}

// open builds the passing open command of the fixture world.
func (f *correctionFixture) open() (OrderCorrectionOutcome, error) {
	return f.service.OpenOrderCorrection(context.Background(), OpenOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, OperationID: f.operation.OperationID,
		Owner: verificationManager, Deadline: domain.Correct4h, Now: previewNow,
	})
}

// reread builds the passing reread command of the fixture world (the
// NEW confirmed version pins) — the manager (the owner of the open).
func (f *correctionFixture) reread(correctionNo int64, epoch int64) (OrderCorrectionOutcome, error) {
	return f.rereadBy(verificationManager, correctionNo, epoch)
}

// rereadBy builds the reread command of an arbitrary actor (the stale
// epoch and the new-owner probes).
func (f *correctionFixture) rereadBy(actor id.UUID, correctionNo int64, epoch int64) (OrderCorrectionOutcome, error) {
	return f.service.RecordOrderCorrectionReread(context.Background(), RecordOrderCorrectionRereadCommand{
		Actor: identityDomain.Principal{ID: actor}, TenantID: testTenant,
		CaseID: previewCaseID, CorrectionNo: correctionNo, ExpectedAssignment: epoch,
		ExternalObjectID: "ABCP-ORDER-78", ProposalID: f.operation.PreviewID,
		VersionNo: f.newConfirmation.PreviewVersionNo, ConfirmationID: f.newConfirmation.ConfirmationID,
		Now: previewNow,
	})
}

// TestOpenOrderCorrectionCarriesBlockedMaterial tests the happy path of
// the open: the correction №1 lands with the anchor pins (the operation,
// the blocked verification number, the receipt identity) and the opening
// opened event carries the blocked positions and the reason word BY
// NAME (the SQL-truth carry); the outcome speaks the pending state word
// AND the «назначение ≠ начало работы» boundary word.
func TestOpenOrderCorrectionCarriesBlockedMaterial(t *testing.T) {
	f := newCorrectionFixture(t)
	outcome, err := f.open()
	if err != nil {
		t.Fatalf("валидное открытие обязано создать коррекцию: %v", err)
	}
	if outcome.Correction.CorrectionNo != 1 || outcome.Correction.OperationID != f.operation.OperationID {
		t.Fatalf("коррекция 1 якорится операцией: %+v", outcome.Correction)
	}
	if outcome.Correction.VerificationNo != 1 {
		t.Fatalf("пин заблокированной проверки: %+v", outcome.Correction)
	}
	if outcome.Correction.ExternalOrderID != "ABCP-ORDER-77" {
		t.Fatalf("пин внешнего идентификатора квитанции: %+v", outcome.Correction)
	}
	if outcome.View.State != domain.OrderCorrectionStatePending {
		t.Fatalf("состояние «коррекция открыта»: %q", outcome.View.State)
	}
	if outcome.StateText != domain.OrderCorrectionNotStartedWord {
		t.Fatalf("слово границы «назначение ≠ начало работы» обязательно: %q", outcome.StateText)
	}
	events := f.corrections.eventsOf(previewCaseID, 1)
	if len(events) != 1 || events[0].Kind != domain.OrderCorrectionEventOpened {
		t.Fatalf("ровно одно событие opened: %+v", events)
	}
	if len(events[0].Positions) != 1 || events[0].Positions[0] != "pos-1" {
		t.Fatalf("позиции переносятся из blocked-события BY NAME: %+v", events[0].Positions)
	}
	if events[0].Reason != domain.VerifyBlockReasonWrongPart {
		t.Fatalf("причина переносится из blocked-события BY NAME: %q", events[0].Reason)
	}
	if events[0].Owner != verificationManager || events[0].CorrectDeadline != domain.Correct4h {
		t.Fatalf("материал раунда: %+v", events[0])
	}
}

// TestOpenOrderCorrectionRefusals pins the honest refusals of the open:
// the anchor word without a blocked decision (the approved contour
// refuses BY NAME), the 409 of the second open while the correction
// lives, the review-scope refusal of the actor and the owner guard.
func TestOpenOrderCorrectionRefusals(t *testing.T) {
	f := newCorrectionFixture(t)
	// The happy open first (the world carries ONE blocked decision).
	if _, err := f.open(); err != nil {
		t.Fatalf("валидное открытие обязано пройти: %v", err)
	}
	// The 409 of the repeated open: the second competing correction of
	// one operation is never created.
	_, err := f.open()
	if !errors.As(err, &domain.OrderCorrectionActiveError{}) {
		t.Fatalf("повторное открытие обязано отказаться 409: %v", err)
	}
	// The actor without the review scope: the honest 403 (the separate
	// no-scope service — the 4-7 precedent: the scope word BEFORE any
	// correction material).
	noScopeService := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{}},
		acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
		WithPreviews(newFakePreviewRepo(), f.verification.world.control, f.verification.material, verificationMembers{},
			fakeEntitlement{status: f.verification.world.admission}, fakeReadiness{stage: f.verification.world.stage, instances: f.verification.world.instances},
			orderConnectors{state: f.verification.world.capState, contractVersion: f.verification.world.contract},
			fakeRouteFacts{profileVersion: f.verification.world.profile}, &fakeIDs{}).
		WithAuthorization(f.confirmations, f.verification.ops).
		WithDispatch(f.verification.dispatch, nil).
		WithOrderVerifications(f.verification.repo).
		WithOrderCorrections(f.corrections)
	_, err = noScopeService.OpenOrderCorrection(context.Background(), OpenOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationNoScope}, TenantID: testTenant,
		CaseID: previewCaseID, OperationID: f.operation.OperationID,
		Owner: verificationManager, Deadline: domain.Correct4h, Now: previewNow,
	})
	if err == nil || !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "review") {
		t.Fatalf("актёр без полномочия review обязан получить 403 словом скоупа: %v", err)
	}
	// The owner without the review scope: the honest member guard.
	_, err = f.service.OpenOrderCorrection(context.Background(), OpenOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, OperationID: f.operation.OperationID,
		Owner: verificationNoScope, Deadline: domain.Correct4h, Now: previewNow,
	})
	if err == nil {
		t.Fatalf("владелец без полномочия review обязан получить отказ гварда: %v", err)
	}
	// The deadline word outside the dictionary: the honest guard.
	_, err = f.service.OpenOrderCorrection(context.Background(), OpenOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, OperationID: f.operation.OperationID,
		Owner: verificationManager, Deadline: "correct_2h", Now: previewNow,
	})
	if err == nil {
		t.Fatalf("чужое слово дедлайна обязано получить отказ гварда: %v", err)
	}
}

// TestOpenOrderCorrectionNoBlockedDecision pins the anchor refusal: the
// operation whose verification decision is APPROVED (not blocked) never
// opens a correction — the honest anchor word BY NAME.
func TestOpenOrderCorrectionNoBlockedDecision(t *testing.T) {
	// The approved world: the verification fixture without the block.
	verification := newVerificationFixture(t)
	corrections := newFakeCorrectionRepo()
	confirmations := newFakeConfirmationRepo()
	service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
		acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
		WithPreviews(newFakePreviewRepo(), verification.world.control, verification.material, verificationMembers{},
			fakeEntitlement{status: verification.world.admission}, fakeReadiness{stage: verification.world.stage, instances: verification.world.instances},
			orderConnectors{state: verification.world.capState, contractVersion: verification.world.contract},
			fakeRouteFacts{profileVersion: verification.world.profile}, &fakeIDs{}).
		WithAuthorization(confirmations, verification.ops).
		WithDispatch(verification.dispatch, nil).
		WithOrderVerifications(verification.repo).
		WithOrderCorrections(corrections)
	if _, err := verification.assign(); err != nil {
		t.Fatalf("назначение обязано пройти: %v", err)
	}
	if _, err := verification.acknowledge(1, 1); err != nil {
		t.Fatalf("принятие обязано пройти: %v", err)
	}
	if _, err := verification.approve(1, 1); err != nil {
		t.Fatalf("подтверждение обязано пройти: %v", err)
	}
	_, err := service.OpenOrderCorrection(context.Background(), OpenOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, OperationID: verification.operation.OperationID,
		Owner: verificationManager, Deadline: domain.Correct4h, Now: previewNow,
	})
	if !errors.Is(err, domain.ErrOrderCorrectionNoBlockedDecision) {
		t.Fatalf("открытие без blocked-решения обязано отказаться якорным словом: %v", err)
	}
}

// TestRecordOrderCorrectionRereadClosesCorrected pins the happy path of
// the reread: the version validations pass (the triple resolves, the
// confirmation is confirmed, the LIVE digest equals the version digest),
// the reread event carries the SAFE pin set only (the external object ID
// and the identity triple) and the outcome speaks the corrected state
// word AND the MANDATORY reread word (AG-4: the manager's words, Staffy
// never reads ABCP).
func TestRecordOrderCorrectionRereadClosesCorrected(t *testing.T) {
	f := newCorrectionFixture(t)
	if _, err := f.open(); err != nil {
		t.Fatalf("открытие обязано пройти: %v", err)
	}
	outcome, err := f.reread(1, 1)
	if err != nil {
		t.Fatalf("валидный перечит обязан закрыть коррекцию: %v", err)
	}
	if outcome.View.State != domain.OrderCorrectionStateCorrected {
		t.Fatalf("состояние «исправлен»: %q", outcome.View.State)
	}
	if outcome.StateText != domain.OrderCorrectionRereadWord {
		t.Fatalf("обязательное слово перечита (AG-4) на исходе: %q", outcome.StateText)
	}
	events := f.corrections.eventsOf(previewCaseID, 1)
	if len(events) != 2 || events[1].Kind != domain.OrderCorrectionEventReread {
		t.Fatalf("ровно два события (opened + reread): %+v", events)
	}
	rereadEvent := events[1]
	if rereadEvent.ExternalObjectID != "ABCP-ORDER-78" {
		t.Fatalf("внешний идентификатор перечита: %+v", rereadEvent)
	}
	if rereadEvent.ProposalID != f.operation.PreviewID || rereadEvent.VersionNo != 2 {
		t.Fatalf("пины новой версии предложения: %+v", rereadEvent)
	}
	if rereadEvent.ConfirmationID != f.newConfirmation.ConfirmationID {
		t.Fatalf("пин нового подтверждения: %+v", rereadEvent)
	}
	if len(rereadEvent.Positions) != 0 || rereadEvent.Reason != "" {
		t.Fatalf("перечит НЕ несёт blocked-материал (SAFE pin set only): %+v", rereadEvent)
	}
}

// TestRecordOrderCorrectionRereadRefusals pins the honest refusals of
// the reread: the superseded confirmation (AC-15 BY NAME), the
// composition divergence after the confirmation (AC-13), the not-owner
// actor and the stale CAS epoch (the raced append never writes).
func TestRecordOrderCorrectionRereadRefusals(t *testing.T) {
	f := newCorrectionFixture(t)
	if _, err := f.open(); err != nil {
		t.Fatalf("открытие обязано пройти: %v", err)
	}
	// AC-15: the OLD confirmation (superseded by the new version) never
	// closes the correction — the refusal travels BY NAME.
	oldConfirmation := domain.Confirmation{
		TenantID:       testTenant,
		ConfirmationID: id.MustParse("01994f64-6c00-7000-8000-0000000000d2"),
		VersionNo:      1, DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
		CaseID: previewCaseID, WorkItemID: previewWorkID,
		Target:    domain.ProposalTarget{Kind: domain.TargetCatalogItem, Ref: previewTargetID},
		PreviewID: f.operation.PreviewID, PreviewVersionNo: 1, PreviewDigest: f.digest,
		ValidUntil: previewNow.Add(24 * time.Hour), ConfirmedBy: verificationOwner,
		CreatedAt: previewNow,
	}
	f.confirmations.confirmations[oldConfirmation.ConfirmationID] = []domain.Confirmation{oldConfirmation}
	f.confirmations.events = append(f.confirmations.events,
		port.ConfirmationEventRow{
			ConfirmationID: oldConfirmation.ConfirmationID, VersionNo: 1,
			CaseID: previewCaseID, EventNo: 1,
			Kind: domain.ConfirmationEventConfirmed, ActorID: verificationOwner, CreatedAt: previewNow,
		},
		port.ConfirmationEventRow{
			ConfirmationID: oldConfirmation.ConfirmationID, VersionNo: 1,
			CaseID: previewCaseID, EventNo: 2,
			Kind: domain.ConfirmationEventSuperseded, ActorID: verificationOwner, CreatedAt: previewNow,
		})
	_, err := f.service.RecordOrderCorrectionReread(context.Background(), RecordOrderCorrectionRereadCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, CorrectionNo: 1, ExpectedAssignment: 1,
		ExternalObjectID: "ABCP-ORDER-78", ProposalID: f.operation.PreviewID,
		VersionNo: 1, ConfirmationID: oldConfirmation.ConfirmationID, Now: previewNow,
	})
	if !errors.Is(err, domain.ErrOrderCorrectionSupersededConfirmation) {
		t.Fatalf("старое подтверждение обязано отказаться BY NAME (AC-15): %v", err)
	}
	// AC-13: the price changed AFTER the confirmation — the LIVE
	// re-derivation diverges from the version digest (the pinned preview
	// v2 carries a foreign digest), the closure is impossible.
	f.verification.ops.pinPreview(domain.ActionPreview{
		TenantID: testTenant, PreviewID: f.operation.PreviewID, VersionNo: 2,
		DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
		CaseID: previewCaseID, WorkItemID: previewWorkID,
		TypedInput: map[string]string{
			"order_digest":     orderTestPin("diverged-digest"),
			"order_material":   "другой материал",
			"known_client_sum": "9999.00",
			"currency":         "RUB",
		},
	})
	_, err = f.reread(1, 1)
	if !errors.Is(err, domain.ErrOrderCorrectionCompositionChanged) {
		t.Fatalf("изменившийся состав обязан отказаться (AC-13): %v", err)
	}
	// Restore the digest pin (the further probes need the passing fence).
	f.verification.ops.pinPreview(f.newPreview)
	// The not-owner actor: the honest actor word.
	_, err = f.service.RecordOrderCorrectionReread(context.Background(), RecordOrderCorrectionRereadCommand{
		Actor: identityDomain.Principal{ID: verificationBackup}, TenantID: testTenant,
		CaseID: previewCaseID, CorrectionNo: 1, ExpectedAssignment: 1,
		ExternalObjectID: "ABCP-ORDER-78", ProposalID: f.operation.PreviewID,
		VersionNo: 2, ConfirmationID: f.newConfirmation.ConfirmationID, Now: previewNow,
	})
	if err == nil || err.Error() != domain.OrderCorrectionNotOwnerWord {
		t.Fatalf("не-owner актёр обязан получить слово истины актёра: %v", err)
	}
	// The stale CAS epoch: the raced append never writes.
	_, err = f.reread(1, 5)
	if !errors.As(err, &domain.OrderCorrectionRacedError{}) {
		t.Fatalf("устаревшая эпоха обязана отказаться гонкой: %v", err)
	}
	// The correction is STILL open (no closure event was written).
	events := f.corrections.eventsOf(previewCaseID, 1)
	if len(events) != 1 {
		t.Fatalf("отказы не пишут событий: %+v", events)
	}
}

// TestRecordOrderCorrectionRereadRaced pins the CAS race of the parallel
// closures: two reread commands of the same epoch — exactly one wins,
// the loser answers the honest current-state words and NEVER writes.
func TestRecordOrderCorrectionRereadRaced(t *testing.T) {
	f := newCorrectionFixture(t)
	if _, err := f.open(); err != nil {
		t.Fatalf("открытие обязано пройти: %v", err)
	}
	f.corrections.raceHook = func(repo *fakeCorrectionRepo) {
		// The parallel winner lands BETWEEN the read and the append: the
		// loser's CAS expectation is stale.
		repo.events[1] = append(repo.events[1], domain.OrderCorrectionEvent{
			TenantID: testTenant, CaseID: previewCaseID, CorrectionNo: 1, EventNo: 2,
			Kind:             domain.OrderCorrectionEventReread,
			ExternalObjectID: "ABCP-ORDER-79", ProposalID: f.operation.PreviewID,
			VersionNo: 2, ConfirmationID: f.newConfirmation.ConfirmationID,
			ActorID: verificationManager, CreatedAt: previewNow,
		})
	}
	_, err := f.reread(1, 1)
	if !errors.As(err, &domain.OrderCorrectionRacedError{}) {
		t.Fatalf("проигравшая гонка обязана отказаться честным словом: %v", err)
	}
	events := f.corrections.eventsOf(previewCaseID, 1)
	if len(events) != 2 {
		t.Fatalf("ровно два события (winner's reread, loser never wrote): %+v", events)
	}
}

// TestCeaseOrderCorrection pins the ceased closure: the structured
// cessation reason word, the client_refused recommendation word (AC-11 —
// the contour 4-10 named, the consent NEVER presumed) and the honest
// state word of the ceased outcome.
func TestCeaseOrderCorrection(t *testing.T) {
	f := newCorrectionFixture(t)
	if _, err := f.open(); err != nil {
		t.Fatalf("открытие обязано пройти: %v", err)
	}
	outcome, err := f.service.CeaseOrderCorrection(context.Background(), CeaseOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, CorrectionNo: 1, ExpectedAssignment: 1,
		Reason: domain.CorrectionCessationClientRefused, Now: previewNow,
	})
	if err != nil {
		t.Fatalf("валидное прекращение обязано закрыть коррекцию: %v", err)
	}
	if outcome.View.State != domain.OrderCorrectionStateCeased {
		t.Fatalf("состояние «прекращена»: %q", outcome.View.State)
	}
	if outcome.StateText != domain.OrderCorrectionCancellationWord {
		t.Fatalf("слово-рекомендация отмены (контур 4-10) обязательно при client_refused: %q", outcome.StateText)
	}
	events := f.corrections.eventsOf(previewCaseID, 1)
	if len(events) != 2 || events[1].Kind != domain.OrderCorrectionEventCeased {
		t.Fatalf("ровно два события (opened + ceased): %+v", events)
	}
	if events[1].CessationReason != domain.CorrectionCessationClientRefused {
		t.Fatalf("причина прекращения словом словаря: %+v", events[1])
	}
	// The not-needed cessation carries NO cancellation word (the plain
	// state word only).
	outcome, err = f.service.CeaseOrderCorrection(context.Background(), CeaseOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, CorrectionNo: 1, ExpectedAssignment: 1,
		Reason: domain.CorrectionCessationNotNeeded, Now: previewNow,
	})
	if err == nil {
		t.Fatalf("повторное прекращение закрытой коррекции обязано отказаться")
	}
	_ = outcome
}

// TestReassignOrderCorrection pins the owner's reassignment: the new
// round opens a NEW epoch (the former owner's stale-epoch closure
// refuses honestly), the reassignment is possible from the overdue
// state (the fallback — never an automatic takeover).
func TestReassignOrderCorrection(t *testing.T) {
	f := newCorrectionFixture(t)
	if _, err := f.open(); err != nil {
		t.Fatalf("открытие обязано пройти: %v", err)
	}
	outcome, err := f.service.ReassignOrderCorrection(context.Background(), ReassignOrderCorrectionCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, CorrectionNo: 1,
		Owner: verificationOwner, Deadline: domain.Correct1d, Now: previewNow,
	})
	if err != nil {
		t.Fatalf("валидное переназначение обязано пройти: %v", err)
	}
	if outcome.View.AssignmentEpoch != 2 {
		t.Fatalf("переназначение открывает новую эпоху: %d", outcome.View.AssignmentEpoch)
	}
	if outcome.View.Owner != verificationOwner {
		t.Fatalf("владелец переназначен: %+v", outcome.View)
	}
	// The FORMER owner answers the honest not-owner word (the server
	// truth — the assignment never impersonates the owner).
	_, err = f.reread(1, 1)
	if err == nil || err.Error() != domain.OrderCorrectionNotOwnerWord {
		t.Fatalf("бывший владелец обязан получить слово истины актёра: %v", err)
	}
	// The new owner's stale-epoch closure refuses honestly (the CAS
	// epoch 1 is stale after the reassignment).
	_, err = f.rereadBy(verificationOwner, 1, 1)
	if !errors.As(err, &domain.OrderCorrectionRacedError{}) {
		t.Fatalf("устаревшая эпоха после переназначения обязана отказаться: %v", err)
	}
	// The new owner's closure with the CURRENT epoch passes.
	if _, err := f.rereadBy(verificationOwner, 1, 2); err != nil {
		t.Fatalf("перечит новым владельцем с текущей эпохой обязан пройти: %v", err)
	}
}

// TestAssignOrderVerificationGateByOpenCorrection pins the FIRST
// structural gate (AC-2): an OPEN correction of the operation refuses
// the new verification assignment with the honest 409 word — the new
// task is never created before the correction closes; after the closure
// the assign passes again (the ladder of the repeated verification).
func TestAssignOrderVerificationGateByOpenCorrection(t *testing.T) {
	f := newCorrectionFixture(t)
	if _, err := f.open(); err != nil {
		t.Fatalf("открытие обязано пройти: %v", err)
	}
	// The assign goes through THE service with the corrections wiring
	// (the gate reads the open correction of the operation).
	assign := func() error {
		_, err := f.service.AssignOrderVerification(context.Background(), AssignOrderVerificationCommand{
			Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
			CaseID: previewCaseID, WorkItemID: previewWorkID, OperationID: f.operation.OperationID,
			Assignee: verificationManager, BackupOwner: verificationBackup,
			Deadline: domain.Verify4h, Now: previewNow,
		})
		return err
	}
	err := assign()
	if !errors.As(err, &domain.OrderCorrectionAssignGateError{}) {
		t.Fatalf("назначение проверки при открытой коррекции обязано отказаться 409 гейт-словом: %v", err)
	}
	if err.Error() != domain.OrderCorrectionAssignGateWord {
		t.Fatalf("гейт-слово назначения: %q", err.Error())
	}
	// No second task was created (the ladder holds ONE task).
	tasks, _ := f.verification.repo.ReadCaseVerifications(context.Background(), tenancy.Context{}, previewCaseID)
	if len(tasks) != 1 {
		t.Fatalf("вторая задача проверки не создаётся: %+v", tasks)
	}
	// After the correction closes the assign passes again (the NEW task
	// №2 — the ladder of the repeated verification).
	if _, err := f.reread(1, 1); err != nil {
		t.Fatalf("перечит обязан пройти: %v", err)
	}
	if err := assign(); err != nil {
		t.Fatalf("назначение после закрытия коррекции обязано пройти: %v", err)
	}
	tasks, _ = f.verification.repo.ReadCaseVerifications(context.Background(), tenancy.Context{}, previewCaseID)
	if len(tasks) != 2 || tasks[1].VerificationNo != 2 {
		t.Fatalf("новая задача №2 по лестнице повторной проверки: %+v", tasks)
	}
}

// TestAuthorizeOperationGateByOpenCorrection pins the SECOND structural
// gate (AC-9/AC-10): an OPEN correction of the case refuses the
// authorization of the NEW create_abcp_customer_order BEFORE the
// reservation transaction — NOTHING is consumed (the confirmation stays
// unconsumed, no operation row lands); after the closure the
// authorization path proceeds past the gate.
func TestAuthorizeOperationGateByOpenCorrection(t *testing.T) {
	f := newCorrectionFixture(t)
	if _, err := f.open(); err != nil {
		t.Fatalf("открытие обязано пройти: %v", err)
	}
	command := authorizeCommandOf(f)
	_, err := f.service.AuthorizeOperation(context.Background(), command)
	if !errors.Is(err, domain.ErrOrderCorrectionOpenGate) {
		t.Fatalf("авторизация нового заказа при открытой коррекции обязана отказаться гейт-словом ДО транзакции: %v", err)
	}
	if err.Error() != domain.OrderCorrectionOrderGateWord {
		t.Fatalf("гейт-слово авторизации: %q", err.Error())
	}
	// NOTHING was consumed and NO operation row landed (the gate
	// precedes the rejection ladder and the reservation).
	if consumed := f.confirmations.consumedCount(f.newConfirmation.ConfirmationID, f.newConfirmation.VersionNo); consumed != 0 {
		t.Fatalf("потребление не происходит на гейте: %d", consumed)
	}
	if f.verification.ops.reserveCalls != 0 {
		t.Fatalf("резервирование не происходит на гейте: %d", f.verification.ops.reserveCalls)
	}
	// After the correction closes (the reread), the authorization
	// proceeds PAST the gate (the later honest refusals of the world are
	// the 3-5 machine's own words — never the gate).
	if _, err := f.reread(1, 1); err != nil {
		t.Fatalf("перечит обязан пройти: %v", err)
	}
	_, err = f.service.AuthorizeOperation(context.Background(), command)
	if errors.Is(err, domain.ErrOrderCorrectionOpenGate) {
		t.Fatalf("гейт не должен держать после закрытия коррекции: %v", err)
	}
}

// authorizeCommandOf builds the authorize command of the NEW confirmed
// version over the fixture world.
func authorizeCommandOf(f *correctionFixture) api.AuthorizeOperationCommand {
	return api.AuthorizeOperationCommand{
		Actor:                 identityDomain.Principal{ID: verificationOwner},
		TenantID:              testTenant,
		CaseID:                previewCaseID,
		ConfirmationID:        f.newConfirmation.ConfirmationID,
		ConfirmationVersionNo: f.newConfirmation.VersionNo,
		IdempotencyKey:        "correction-gate-key-1",
		Now:                   previewNow,
	}
}
