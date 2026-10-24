package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"fmt"

	"github.com/artemiosu/staffy/internal/action/domain"
	"github.com/artemiosu/staffy/internal/action/port"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.7 step-02: the manager order verification application contour
// (the five commands, the decision fence, the CAS races)
// ---------------------------------------------------------------------------

// verificationTestPin derives a deterministic 64-hex pin for the fixtures.
func verificationTestPin(label string) string {
	return orderTestPin(label)
}

// The fixture identities of the verification world: the owner (the actor
// with the review scope — the assigner), the manager (the assignee with
// the review scope — the pilot's single figure), the backup member and
// the operator WITHOUT the review scope (the honest refusal probes).
var (
	verificationOwner   = id.MustParse("01994f64-6c00-7000-8000-0000000000b1")
	verificationManager = id.MustParse("01994f64-6c00-7000-8000-0000000000b2")
	verificationBackup  = id.MustParse("01994f64-6c00-7000-8000-0000000000b3")
	verificationNoScope = id.MustParse("01994f64-6c00-7000-8000-0000000000b4")
)

// verificationMembers serves the LIVE member facts of the world: the
// owner (all scopes), the manager (review via expert_qa), the backup
// (operator — a member, no review needed for the backup role) and the
// scope-less operator (the refusal probe).
type verificationMembers struct{}

func (verificationMembers) MembersFacts(_ context.Context, _ identityDomain.Principal, _ id.UUID) ([]identityDomain.Membership, error) {
	owner, manager, backup, noScope := verificationOwner, verificationManager, verificationBackup, verificationNoScope
	return []identityDomain.Membership{
		{TenantID: testTenant, PrincipalID: &owner, Email: "owner@staffy.test",
			Roles: []identityDomain.Role{identityDomain.Owner}, Status: "active", Version: 1},
		{TenantID: testTenant, PrincipalID: &manager, Email: "manager@staffy.test",
			Roles: []identityDomain.Role{identityDomain.ExpertQA}, Status: "active", Version: 1},
		{TenantID: testTenant, PrincipalID: &backup, Email: "backup@staffy.test",
			Roles: []identityDomain.Role{identityDomain.Operator}, Status: "active", Version: 1},
		{TenantID: testTenant, PrincipalID: &noScope, Email: "noscope@staffy.test",
			Roles: []identityDomain.Role{identityDomain.Operator}, Status: "active", Version: 1},
	}, nil
}

// fakeVerificationRepo is the in-memory OrderVerificationRepository with
// the CAS semantics of the store: the per-case verification_no ladder,
// the one-active-task rule, the epoch re-validation under the append and
// the raceHook (the parallel-winner simulation of the CAS tests).
type fakeVerificationRepo struct {
	tasks      []domain.OrderVerification
	events     map[int64][]domain.OrderVerificationEvent
	nextNumber map[id.UUID]int64
	raceHook   func(repo *fakeVerificationRepo)
}

func newFakeVerificationRepo() *fakeVerificationRepo {
	return &fakeVerificationRepo{events: map[int64][]domain.OrderVerificationEvent{}, nextNumber: map[id.UUID]int64{}}
}

func (f *fakeVerificationRepo) eventsOf(caseID id.UUID, verificationNo int64) []domain.OrderVerificationEvent {
	out := []domain.OrderVerificationEvent{}
	for _, event := range f.events[verificationNo] {
		if event.CaseID == caseID {
			out = append(out, event)
		}
	}
	return out
}

func (f *fakeVerificationRepo) activeOf(operationID id.UUID, now time.Time) bool {
	for _, task := range f.tasks {
		if task.OperationID != operationID {
			continue
		}
		if domain.DeriveOrderVerificationState(task, f.eventsOf(task.CaseID, task.VerificationNo), now).Active() {
			return true
		}
	}
	return false
}

func (f *fakeVerificationRepo) ReadOperationVerifications(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]domain.OrderVerification, error) {
	out := []domain.OrderVerification{}
	for _, task := range f.tasks {
		if task.OperationID == operationID {
			out = append(out, task)
		}
	}
	return out, nil
}

func (f *fakeVerificationRepo) ReadCaseVerifications(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]domain.OrderVerification, error) {
	out := []domain.OrderVerification{}
	for _, task := range f.tasks {
		if task.CaseID == caseID {
			out = append(out, task)
		}
	}
	return out, nil
}

func (f *fakeVerificationRepo) ReadVerificationEvents(_ context.Context, _ tenancy.Context, caseID id.UUID, verificationNo int64) ([]domain.OrderVerificationEvent, error) {
	return append([]domain.OrderVerificationEvent(nil), f.eventsOf(caseID, verificationNo)...), nil
}

func (f *fakeVerificationRepo) AppendOrderVerification(_ context.Context, p port.AppendOrderVerificationParams) (domain.OrderVerification, error) {
	if f.activeOf(p.OperationID, p.Now) {
		return domain.OrderVerification{}, port.ErrVerificationActive
	}
	verificationNo := f.nextNumber[p.CaseID] + 1
	f.nextNumber[p.CaseID] = verificationNo
	task := domain.OrderVerification{
		TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
		VerificationNo: verificationNo, OperationID: p.OperationID,
		ReceiptVersionNo: p.ReceiptVersionNo, ExternalOrderID: p.ExternalOrderID,
		CreatorID: p.CreatorID, CreatedAt: p.Now,
	}
	f.tasks = append(f.tasks, task)
	event := domain.OrderVerificationEvent{
		TenantID: task.TenantID, CaseID: task.CaseID, VerificationNo: verificationNo, EventNo: 1,
		Kind: domain.OrderVerificationEventAssigned, Assignee: p.Assignee, BackupOwner: p.BackupOwner,
		VerifyDeadline: p.VerifyDeadline, DeadlineAt: p.DeadlineAt, ActorID: p.CreatorID, CreatedAt: p.Now,
	}
	f.events[verificationNo] = append(f.events[verificationNo], event)
	return task, nil
}

func (f *fakeVerificationRepo) AppendOrderVerificationEvent(_ context.Context, p port.AppendOrderVerificationEventParams) (domain.OrderVerificationEvent, error) {
	if f.raceHook != nil {
		hook := f.raceHook
		f.raceHook = nil
		hook(f)
	}
	var task *domain.OrderVerification
	for i := range f.tasks {
		if f.tasks[i].CaseID == p.CaseID && f.tasks[i].VerificationNo == p.VerificationNo {
			task = &f.tasks[i]
			break
		}
	}
	if task == nil {
		return domain.OrderVerificationEvent{}, port.ErrVerificationNotFound
	}
	events := f.eventsOf(p.CaseID, p.VerificationNo)
	view := domain.DeriveOrderVerificationState(*task, events, p.Now)
	if view.State != p.ExpectedState || view.AssignmentEpoch != p.ExpectedEpoch {
		return domain.OrderVerificationEvent{}, port.ErrVerificationRaced
	}
	event := p.Event
	event.TenantID = task.TenantID
	event.CaseID = task.CaseID
	event.VerificationNo = task.VerificationNo
	event.EventNo = int64(len(events)) + 1
	event.CreatedAt = p.Now
	f.events[p.VerificationNo] = append(f.events[p.VerificationNo], event)
	return event, nil
}

// verificationFixture is the full world of the verification tests: the
// order case view with ONE selected position, the committed order
// operation with the pinned preview (the digest + the canonical render)
// and the committed receipt with the external ABCP Order ID.
type verificationFixture struct {
	world     previewLiveFixture
	repo      *fakeVerificationRepo
	ops       *fakeOperationRepo
	dispatch  *fakeDispatchRepo
	material  fakeCaseMaterial
	service   *Service
	operation domain.Operation
	digest    string
}

// newVerificationFixture wires the service over the committed order
// world (the unit composition root of the verification tests).
func newVerificationFixture(t *testing.T) *verificationFixture {
	t.Helper()
	world := orderWorld()
	repo := newFakeVerificationRepo()
	ops := newFakeOperationRepo(newFakeConfirmationRepo())
	dispatch := newFakeDispatchRepo()
	material := fakeCaseMaterial{view: world.caseView}
	liveMaterial := mustDeriveOrderMaterial(t, world.caseView)
	digest := domain.OrderDigestOf(liveMaterial)
	render := domain.OrderMaterialCanonicalText(liveMaterial)
	operation := domain.Operation{
		TenantID:     testTenant,
		OperationID:  id.MustParse("01994f64-6c00-7000-8000-0000000000c1"),
		DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
		PreviewID: id.MustParse("01994f64-6c00-7000-8000-0000000000c2"), PreviewVersionNo: 1,
		ConfirmationID: id.MustParse("01994f64-6c00-7000-8000-0000000000c3"), ConfirmationVersionNo: 1,
		CaseID: previewCaseID, WorkItemID: previewWorkID,
		OwnerKind: "human", OwnerRef: verificationOwner, OwnerEpoch: 1,
		Partition: domain.PartitionHumanOnly,
		CreatedAt: previewNow,
	}
	ops.operations = []domain.Operation{operation}
	ops.ledgers[operation.OperationID] = []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, At: previewNow},
		{EntryNo: 2, Kind: domain.LedgerEntrySubmitted, At: previewNow},
		{EntryNo: 3, Kind: domain.LedgerEntryCommitted, At: previewNow},
	}
	ops.pinPreview(domain.ActionPreview{
		TenantID: testTenant, PreviewID: operation.PreviewID, VersionNo: 1,
		DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
		CaseID: previewCaseID, WorkItemID: previewWorkID,
		TypedInput: map[string]string{
			"order_digest":     digest,
			"order_material":   render,
			"known_client_sum": "4200.50",
			"currency":         "RUB",
		},
	})
	dispatch.receipts[operation.OperationID] = []port.RecordDispatchOutcomeParams{{
		ReceiptVersionNo: 1, ReceiptWord: domain.ReceiptOutcomeCommitted,
		ReceiptReferences: port.ReceiptReferences{
			CaseID: previewCaseID, WorkItemID: previewWorkID,
			PreviewID: operation.PreviewID, PreviewVersionNo: 1,
			ConfirmationID: operation.ConfirmationID, ConfirmationVersionNo: 1,
			DefinitionID: domain.OrderActionID, DefinitionVersion: domain.OrderActionVersion,
			ExternalObjectID: "ABCP-ORDER-77", ExternalObjectVersion: "v1",
		},
	}}
	service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
		acceptAllPacks{}, acceptAllCaps{}, nil).
		WithPreviews(newFakePreviewRepo(), world.control, material, verificationMembers{},
			fakeEntitlement{status: world.admission}, fakeReadiness{stage: world.stage, instances: world.instances},
			orderConnectors{state: world.capState, contractVersion: world.contract},
			fakeRouteFacts{profileVersion: world.profile}, &fakeIDs{}).
		WithAuthorization(newFakeConfirmationRepo(), ops).
		WithDispatch(dispatch, nil).
		WithOrderVerifications(repo)
	return &verificationFixture{world: world, repo: repo, ops: ops, dispatch: dispatch,
		material: material, service: service, operation: operation, digest: digest}
}

// mustDeriveOrderMaterial derives the live material of the fixture view
// (the test helper — the derivation itself is pinned by the 4-6 tests).
func mustDeriveOrderMaterial(t *testing.T, view casesAPI.CaseView) domain.OrderCommitmentMaterial {
	t.Helper()
	material, err := DeriveOrderCommitmentMaterial(view)
	if err != nil {
		t.Fatalf("фixture view обязан деривировать материал заказа: %v", err)
	}
	return material
}

// confirmations exposes the confirmation repository of the fixture
// (the non-member probe service wires its own copy).
func (f *verificationFixture) confirmations() port.ConfirmationRepository {
	return newFakeConfirmationRepo()
}

// assign builds the passing assign command of the fixture world.
func (f *verificationFixture) assign() (OrderVerificationOutcome, error) {
	return f.service.AssignOrderVerification(context.Background(), AssignOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, WorkItemID: previewWorkID, OperationID: f.operation.OperationID,
		Assignee: verificationManager, BackupOwner: verificationBackup,
		Deadline: domain.Verify4h, Now: previewNow,
	})
}

// acknowledge builds the passing acknowledge command of the assignee.
func (f *verificationFixture) acknowledge(verificationNo int64, epoch int64) (OrderVerificationOutcome, error) {
	return f.service.AcknowledgeOrderVerification(context.Background(), AcknowledgeOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: verificationNo, ExpectedAssignment: epoch, Now: previewNow,
	})
}

// approve builds the passing approve command of the assignee.
func (f *verificationFixture) approve(verificationNo int64, epoch int64) (OrderVerificationOutcome, error) {
	return f.service.ApproveOrderVerification(context.Background(), ApproveOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: verificationNo,
		ExpectedAssignment: epoch, ExpectedOrderDigest: f.digest, Now: previewNow,
	})
}

// TestAssignOrderVerificationCreatesTaskOne tests the happy path: the
// task v1 + the opening assigned event land in ONE transaction; the
// outcome speaks the assigned state word AND the delivery boundary word
// («назначен ≠ доставлено ≠ принял» — the assignment is never the
// acceptance, the external channel does not exist).
func TestAssignOrderVerificationCreatesTaskOne(t *testing.T) {
	f := newVerificationFixture(t)
	outcome, err := f.assign()
	if err != nil {
		t.Fatalf("валидное назначение обязано создать задачу: %v", err)
	}
	if outcome.Task.VerificationNo != 1 || outcome.Task.OperationID != f.operation.OperationID {
		t.Fatalf("задача 1 якорится операцией: %+v", outcome.Task)
	}
	if outcome.Task.ReceiptVersionNo != 1 || outcome.Task.ExternalOrderID != "ABCP-ORDER-77" {
		t.Fatalf("пины квитанции: %+v", outcome.Task)
	}
	if outcome.View.State != domain.OrderVerificationStateAssigned {
		t.Fatalf("состояние «назначен»: %q", outcome.View.State)
	}
	if outcome.StateText != domain.OrderVerificationDeliveryWord {
		t.Fatalf("слово границы доставки обязательно на исходе назначения: %q", outcome.StateText)
	}
	events := f.repo.eventsOf(previewCaseID, 1)
	if len(events) != 1 || events[0].Kind != domain.OrderVerificationEventAssigned {
		t.Fatalf("ровно одно событие assigned: %+v", events)
	}
	if events[0].Assignee != verificationManager || events[0].BackupOwner != verificationBackup {
		t.Fatalf("материал раунда: %+v", events[0])
	}
	if events[0].VerifyDeadline != domain.Verify4h {
		t.Fatalf("слово дедлайна: %q", events[0].VerifyDeadline)
	}
}

// TestAssignOrderVerificationAnchorRefusalsByName pins the anchor
// refusals BY NAME: the non-order operation, the non-committed state and
// the receipt without the external ID all answer the honest anchor word
// (the task is built over the CREATED order only).
func TestAssignOrderVerificationAnchorRefusalsByName(t *testing.T) {
	f := newVerificationFixture(t)
	// The non-order definition.
	f.ops.operations[0].DefinitionID = "quote.send-price-quote"
	if _, err := f.assign(); !errors.Is(err, domain.ErrOrderVerificationNoReceipt) {
		t.Fatalf("не-заказная операция — отказ якоря, получено: %v", err)
	}
	f.ops.operations[0].DefinitionID = domain.OrderActionID
	// The non-committed state (dispatch_blocked parking — the AG-4 word
	// already lives on the 4-6 panel).
	f.ops.ledgers[f.operation.OperationID] = []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, At: previewNow},
		{EntryNo: 2, Kind: domain.LedgerEntryDispatchBlocked, At: previewNow},
	}
	if _, err := f.assign(); !errors.Is(err, domain.ErrOrderVerificationNoReceipt) {
		t.Fatalf("незакоммиченная операция — отказ якоря, получено: %v", err)
	}
	f.ops.ledgers[f.operation.OperationID] = []domain.OperationLedgerEntry{
		{EntryNo: 1, Kind: domain.LedgerEntryAuthorized, At: previewNow},
		{EntryNo: 2, Kind: domain.LedgerEntryCommitted, At: previewNow},
	}
	// The receipt without the external ABCP Order ID.
	f.dispatch.receipts[f.operation.OperationID] = []port.RecordDispatchOutcomeParams{{
		ReceiptVersionNo: 1, ReceiptWord: domain.ReceiptOutcomeCommitted,
	}}
	if _, err := f.assign(); !errors.Is(err, domain.ErrOrderVerificationNoReceipt) {
		t.Fatalf("квитанция без внешнего ID — отказ якоря, получено: %v", err)
	}
	// Nothing landed anywhere.
	if len(f.repo.tasks) != 0 {
		t.Fatalf("отказы якоря не пишут задач, записано %d", len(f.repo.tasks))
	}
}

// TestAssignOrderVerificationActiveConflictByName pins the honest 409 of
// the repeated assign: the active task refuses the second competing task
// with the CURRENT lifecycle words (the HandoffActiveError precedent).
func TestAssignOrderVerificationActiveConflictByName(t *testing.T) {
	f := newVerificationFixture(t)
	if _, err := f.assign(); err != nil {
		t.Fatalf("первое назначение обязано пройти: %v", err)
	}
	_, err := f.assign()
	var active domain.OrderVerificationActiveError
	if !errors.As(err, &active) {
		t.Fatalf("повторное назначение — конфликт активной задачи, получено: %v", err)
	}
	if active.State != domain.OrderVerificationStateAssigned {
		t.Fatalf("конфликт несёт текущее слово состояния: %q", active.State)
	}
	if active.StateText != domain.OrderVerificationStateText(domain.OrderVerificationStateAssigned) {
		t.Fatalf("конфликт несёт честный текст состояния: %q", active.StateText)
	}
	if len(f.repo.tasks) != 1 {
		t.Fatalf("вторая конкурирующая задача не создаётся, задач %d", len(f.repo.tasks))
	}
}

// TestAssignOrderVerificationScopeAndMemberRefusals pins the rights
// ladder: the non-member 404, the member without the review scope 403
// (the words BEFORE any material), the assignee without the review scope
// and the non-member backup.
func TestAssignOrderVerificationScopeAndMemberRefusals(t *testing.T) {
	f := newVerificationFixture(t)
	nonMember := New(scopeIdentity{member: false}, acceptAllPacks{}, acceptAllCaps{}, nil).
		WithPreviews(newFakePreviewRepo(), f.world.control, f.material, verificationMembers{},
			fakeEntitlement{status: f.world.admission}, fakeReadiness{stage: f.world.stage, instances: f.world.instances},
			orderConnectors{state: f.world.capState, contractVersion: f.world.contract},
			fakeRouteFacts{profileVersion: f.world.profile}, &fakeIDs{}).
		WithAuthorization(f.confirmations(), f.ops).
		WithDispatch(f.dispatch, nil).
		WithOrderVerifications(f.repo)
	_, err := nonMember.AssignOrderVerification(context.Background(), AssignOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, WorkItemID: previewWorkID, OperationID: f.operation.OperationID,
		Assignee: verificationManager, BackupOwner: verificationBackup, Deadline: domain.Verify4h, Now: previewNow,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("не-участник — честный 404, получено: %v", err)
	}
	noReview := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Operations: true}},
		acceptAllPacks{}, acceptAllCaps{}, nil).
		WithPreviews(newFakePreviewRepo(), f.world.control, f.material, verificationMembers{},
			fakeEntitlement{status: f.world.admission}, fakeReadiness{stage: f.world.stage, instances: f.world.instances},
			orderConnectors{state: f.world.capState, contractVersion: f.world.contract},
			fakeRouteFacts{profileVersion: f.world.profile}, &fakeIDs{}).
		WithAuthorization(newFakeConfirmationRepo(), f.ops).
		WithDispatch(f.dispatch, nil).
		WithOrderVerifications(f.repo)
	_, err = noReview.AssignOrderVerification(context.Background(), AssignOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationNoScope}, TenantID: testTenant,
		CaseID: previewCaseID, WorkItemID: previewWorkID, OperationID: f.operation.OperationID,
		Assignee: verificationManager, BackupOwner: verificationBackup, Deadline: domain.Verify4h, Now: previewNow,
	})
	if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "review") {
		t.Fatalf("участник без review — 403 словом скоупа, получено: %v", err)
	}
	// The assignee without the review scope (the operator of the grant
	// matrix): the honest member refusal.
	_, err = f.service.AssignOrderVerification(context.Background(), AssignOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, WorkItemID: previewWorkID, OperationID: f.operation.OperationID,
		Assignee: verificationNoScope, BackupOwner: verificationBackup, Deadline: domain.Verify4h, Now: previewNow,
	})
	if err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("assignee без review — отказ словом, получено: %v", err)
	}
	// The unknown-word deadline.
	_, err = f.service.AssignOrderVerification(context.Background(), AssignOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, WorkItemID: previewWorkID, OperationID: f.operation.OperationID,
		Assignee: verificationManager, BackupOwner: verificationBackup, Deadline: "ack_15m", Now: previewNow,
	})
	if err == nil || !strings.Contains(err.Error(), "verify") {
		t.Fatalf("чужое слово дедлайна — отказ словарём, получено: %v", err)
	}
}

// TestAcknowledgeOrderVerificationAcceptsByAssignee pins the acceptance:
// the assignee's explicit act moves the state to acknowledged; the
// not-assignee actor (even the task owner) refuses with the honest word;
// the raced epoch refuses with the current-state words.
func TestAcknowledgeOrderVerificationAcceptsByAssignee(t *testing.T) {
	f := newVerificationFixture(t)
	if _, err := f.assign(); err != nil {
		t.Fatalf("назначение обязано пройти: %v", err)
	}
	// The owner (the assigner) is NOT the assignee: the honest refusal
	// FIRST (the state is still «назначен»).
	_, err := f.service.AcknowledgeOrderVerification(context.Background(), AcknowledgeOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 1, Now: previewNow,
	})
	if err == nil || err.Error() != domain.OrderVerificationNotAssigneeWord {
		t.Fatalf("не-assignee актёр — слово истины, получено: %v", err)
	}
	// The TRUE concurrent race (AC-7): the reassignment lands BETWEEN the
	// assignee's read and the append — the raced command never writes, the
	// honest raced word with the CURRENT state travels to the caller.
	f.repo.raceHook = func(repo *fakeVerificationRepo) {
		repo.events[1] = append(repo.events[1], domain.OrderVerificationEvent{
			TenantID: testTenant, CaseID: previewCaseID, VerificationNo: 1, EventNo: 2,
			Kind:     domain.OrderVerificationEventAssigned,
			Assignee: verificationManager, BackupOwner: verificationBackup,
			VerifyDeadline: domain.Verify1d, DeadlineAt: previewNow.Add(24 * time.Hour),
			ActorID: verificationOwner, CreatedAt: previewNow,
		})
	}
	_, err = f.acknowledge(1, 1)
	var raced domain.OrderVerificationRacedError
	if !errors.As(err, &raced) {
		t.Fatalf("устаревшая эпоха — конфликт гонки, получено: %v", err)
	}
	if raced.State != domain.OrderVerificationStateAssigned {
		t.Fatalf("конфликт гонки несёт текущее слово состояния: %q", raced.State)
	}
	if events := f.repo.eventsOf(previewCaseID, 1); len(events) != 2 {
		t.Fatalf("гонка не пишет события принятия, событий %d", len(events))
	}
	// The acceptance over the CURRENT epoch passes.
	outcome, err := f.acknowledge(1, 2)
	if err != nil {
		t.Fatalf("принятие assignee на текущей эпохе обязано пройти: %v", err)
	}
	if outcome.View.State != domain.OrderVerificationStateAcknowledged {
		t.Fatalf("состояние «принял»: %q", outcome.View.State)
	}
}

// TestApproveOrderVerificationRecordsHumanReceipt pins the happy path of
// the decision: the fence passes (the live digest equals the pin equals
// the command's expectation), the approved event carries the material
// pins of the Human Verification Receipt (the digest, the receipt
// identity, the external ABCP Order ID, the evidence anchors) and the
// outcome carries the MANDATORY manual-contour word.
func TestApproveOrderVerificationRecordsHumanReceipt(t *testing.T) {
	f := newVerificationFixture(t)
	if _, err := f.assign(); err != nil {
		t.Fatalf("назначение обязано пройти: %v", err)
	}
	if _, err := f.acknowledge(1, 1); err != nil {
		t.Fatalf("принятие обязано пройти: %v", err)
	}
	outcome, err := f.approve(1, 1)
	if err != nil {
		t.Fatalf("подтверждение обязано пройти: %v", err)
	}
	if outcome.View.State != domain.OrderVerificationStateApproved {
		t.Fatalf("состояние «проверка подтверждена»: %q", outcome.View.State)
	}
	if outcome.StateText != domain.OrderVerificationManualContourWord {
		t.Fatalf("ОБЯЗАТЕЛЬНОЕ слово ручного контура на исходе: %q", outcome.StateText)
	}
	events := f.repo.eventsOf(previewCaseID, 1)
	if len(events) != 3 || events[2].Kind != domain.OrderVerificationEventApproved {
		t.Fatalf("ровно три события, последнее approved: %+v", events)
	}
	approved := events[2]
	if approved.OrderDigest != f.digest {
		t.Fatalf("пин дайджеста точной версии: %q", approved.OrderDigest)
	}
	if approved.ReceiptVersionNo != 1 || approved.ExternalOrderID != "ABCP-ORDER-77" {
		t.Fatalf("пины квитанции: %+v", approved)
	}
	joined := strings.Join(approved.EvidenceAnchors, " ")
	for _, needle := range []string{"operation:", "work_item:", "receipt:", "position:pos-1"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("якорь evidence %q отсутствует: %v", needle, approved.EvidenceAnchors)
		}
	}
}

// TestApproveOrderVerificationFenceRefusesStaleComposition pins THE
// DECISION FENCE (the dispatch-ladder 4.5 code reused): a re-recorded
// offer price changes the LIVE digest — the decision is refused BY NAME
// (the changed-composition word), the event is NEVER written; the same
// refusal answers the command whose expected digest is not the
// operation's pin and the LIVE derivation refusal (the vanished
// selection).
func TestApproveOrderVerificationFenceRefusesStaleComposition(t *testing.T) {
	f := newVerificationFixture(t)
	if _, err := f.assign(); err != nil {
		t.Fatalf("назначение обязано пройти: %v", err)
	}
	if _, err := f.acknowledge(1, 1); err != nil {
		t.Fatalf("принятие обязано пройти: %v", err)
	}
	// The LIVE price moved: the CURRENT offer record of the chosen
	// supplier carries a new price — the live digest diverges from the
	// pin of the operation.
	f.material.view.Comparison.Records[0].ClientPrice = 4300.00
	if _, err := f.approve(1, 1); !errors.Is(err, domain.ErrOrderVerificationCompositionChanged) {
		t.Fatalf("живой дайджест разошёлся — отказ BY NAME, получено: %v", err)
	}
	if events := f.repo.eventsOf(previewCaseID, 1); len(events) != 2 {
		t.Fatalf("отказ забора НЕ пишет события, событий %d", len(events))
	}
	// The command's expected digest is not the operation's pin (the
	// manager confirmed another version).
	f.material.view = orderWorld().caseView
	_, err := f.service.ApproveOrderVerification(context.Background(), ApproveOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 1,
		ExpectedOrderDigest: verificationTestPin("another-version"), Now: previewNow,
	})
	if !errors.Is(err, domain.ErrOrderVerificationCompositionChanged) {
		t.Fatalf("чужой ожидаемый дайджест — отказ BY NAME, получено: %v", err)
	}
	// The LIVE derivation refuses (the selection vanished): the SAME
	// honest changed-composition word — never a masked pass.
	f.material.view.Comparison.Selections = nil
	if _, err := f.approve(1, 1); !errors.Is(err, domain.ErrOrderVerificationCompositionChanged) {
		t.Fatalf("пропавший выбор — отказ словом состава, получено: %v", err)
	}
}

// TestApproveOrderVerificationWithoutAcknowledgementRefuses pins AC-7:
// the decision without the acceptance refuses with the honest
// «менеджер ещё не принял задачу проверки» word.
func TestApproveOrderVerificationWithoutAcknowledgementRefuses(t *testing.T) {
	f := newVerificationFixture(t)
	if _, err := f.assign(); err != nil {
		t.Fatalf("назначение обязано пройти: %v", err)
	}
	_, err := f.approve(1, 1)
	if err == nil || err.Error() != domain.OrderVerificationNotAcknowledgedWord {
		t.Fatalf("решение без принятия — слово «ещё не принял», получено: %v", err)
	}
}

// TestBlockOrderVerificationRecordsPositionsAndReason pins the
// correction decision: the affected positions (validated against the
// pinned composition of the operation) and the reason word land in the
// blocked event; the outcome carries the correction-launch word; a
// foreign position and a free reason refuse honestly.
func TestBlockOrderVerificationRecordsPositionsAndReason(t *testing.T) {
	f := newVerificationFixture(t)
	if _, err := f.assign(); err != nil {
		t.Fatalf("назначение обязано пройти: %v", err)
	}
	if _, err := f.acknowledge(1, 1); err != nil {
		t.Fatalf("принятие обязано пройти: %v", err)
	}
	// The refusal probes FIRST (the state is still «принял»): the foreign
	// position (outside the pinned composition) and the free reason word.
	_, err := f.service.BlockOrderVerification(context.Background(), BlockOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 1,
		Positions: []string{"pos-9"}, Reason: domain.VerifyBlockReasonWrongPart, Now: previewNow,
	})
	if err == nil || !strings.Contains(err.Error(), "состав операции") {
		t.Fatalf("чужая позиция — отказ словом состава, получено: %v", err)
	}
	_, err = f.service.BlockOrderVerification(context.Background(), BlockOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 1,
		Positions: []string{"pos-1"}, Reason: "expensive", Now: previewNow,
	})
	if err == nil || !strings.Contains(err.Error(), "словар") {
		t.Fatalf("свободная причина — отказ словарём, получено: %v", err)
	}
	if events := f.repo.eventsOf(previewCaseID, 1); len(events) != 2 {
		t.Fatalf("пробы отказов не пишут событий, событий %d", len(events))
	}
	// The passing block.
	outcome, err := f.service.BlockOrderVerification(context.Background(), BlockOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 1,
		Positions: []string{"pos-1"}, Reason: domain.VerifyBlockReasonWrongPart, Now: previewNow,
	})
	if err != nil {
		t.Fatalf("блокировка обязана пройти: %v", err)
	}
	if outcome.View.State != domain.OrderVerificationStateBlocked {
		t.Fatalf("состояние «заблокировано»: %q", outcome.View.State)
	}
	if outcome.StateText != domain.OrderVerificationCorrectionWord {
		t.Fatalf("слово запуска коррекции на исходе: %q", outcome.StateText)
	}
	events := f.repo.eventsOf(previewCaseID, 1)
	if len(events) != 3 || events[2].Kind != domain.OrderVerificationEventBlocked {
		t.Fatalf("последнее событие blocked: %+v", events)
	}
	if len(events[2].Positions) != 1 || events[2].Positions[0] != "pos-1" {
		t.Fatalf("затронутые позиции: %v", events[2].Positions)
	}
	if events[2].Reason != domain.VerifyBlockReasonWrongPart {
		t.Fatalf("слово причины: %q", events[2].Reason)
	}
}

// TestReassignOrderVerificationOpensNewEpoch pins the owner's
// reassignment: a NEW assigned event opens a NEW round with a NEW epoch;
// the former assignee's stale-epoch acknowledge answers the honest
// «задача переназначена» raced word (AC-7); the closed task refuses.
func TestReassignOrderVerificationOpensNewEpoch(t *testing.T) {
	f := newVerificationFixture(t)
	if _, err := f.assign(); err != nil {
		t.Fatalf("назначение обязано пройти: %v", err)
	}
	outcome, reassignErr := f.service.ReassignOrderVerification(context.Background(), ReassignOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1,
		Assignee: verificationOwner, BackupOwner: verificationManager,
		Deadline: domain.Verify1d, Now: previewNow,
	})
	if reassignErr != nil {
		t.Fatalf("переназначение обязано пройти: %v", reassignErr)
	}
	if outcome.View.AssignmentEpoch != 2 {
		t.Fatalf("новая эпоха раунда: %d", outcome.View.AssignmentEpoch)
	}
	if outcome.View.Assignee != verificationOwner {
		t.Fatalf("новый assignee: %q", outcome.View.Assignee)
	}
	// The stale-epoch acceptance of the NEW assignee (the screen epoch 1
	// is outdated — the reassignment opened epoch 2): the honest raced
	// refusal, the command never writes.
	_, err := f.service.AcknowledgeOrderVerification(context.Background(), AcknowledgeOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 1, Now: previewNow,
	})
	var raced domain.OrderVerificationRacedError
	if !errors.As(err, &raced) {
		t.Fatalf("устаревшая эпоха после переназначения — конфликт, получено: %v", err)
	}
	// The new assignee acknowledges over the NEW epoch.
	_, err = f.service.AcknowledgeOrderVerification(context.Background(), AcknowledgeOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1, ExpectedAssignment: 2, Now: previewNow,
	})
	if err != nil {
		t.Fatalf("принятие нового assignee обязано пройти: %v", err)
	}
	// The closed task (after the decision) refuses the reassignment: the
	// NEW task is the ladder, never a rewrite. The decision is the NEW
	// assignee's act (the owner).
	_, decideErr := f.service.ApproveOrderVerification(context.Background(), ApproveOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1,
		ExpectedAssignment: 2, ExpectedOrderDigest: f.digest, Now: previewNow,
	})
	if decideErr != nil {
		t.Fatalf("решение нового assignee обязано пройти: %v", decideErr)
	}
	_, err = f.service.ReassignOrderVerification(context.Background(), ReassignOrderVerificationCommand{
		Actor: identityDomain.Principal{ID: verificationOwner}, TenantID: testTenant,
		CaseID: previewCaseID, VerificationNo: 1,
		Assignee: verificationManager, BackupOwner: verificationBackup,
		Deadline: domain.Verify4h, Now: previewNow,
	})
	if err == nil || !strings.Contains(err.Error(), "закрыта") {
		t.Fatalf("переназначение закрытой задачи — отказ словом, получено: %v", err)
	}
	// The repeated verification of the SAME operation after the decision:
	// the NEW task verification_no=2 (the ladder, AC-14).
	second, err := f.assign()
	if err != nil {
		t.Fatalf("повторная проверка решённой операции — новая задача: %v", err)
	}
	if second.Task.VerificationNo != 2 {
		t.Fatalf("лестница повторной проверки: задача %d", second.Task.VerificationNo)
	}
}

// TestOrderVerificationCommandsCarryNoSupplierSendFields pins the OQ-3
// structural fence: NO command of the contour carries a supplier-send
// field (the reflection probe of the «оператор не вводит» family).
func TestOrderVerificationCommandsCarryNoSupplierSendFields(t *testing.T) {
	for _, command := range []any{
		AssignOrderVerificationCommand{}, AcknowledgeOrderVerificationCommand{},
		ApproveOrderVerificationCommand{}, BlockOrderVerificationCommand{},
		ReassignOrderVerificationCommand{},
	} {
		for _, forbidden := range []string{"SupplierSend", "SendToSupplier", "AutoSend", "Dispatch"} {
			if strings.Contains(reflectTypeName(command), forbidden) {
				t.Fatalf("команда содержит слово отправки: %s", reflectTypeName(command))
			}
		}
	}
}

// reflectTypeName renders the type name of the probe target.
func reflectTypeName(value any) string {
	return fmt.Sprintf("%T", value)
}
