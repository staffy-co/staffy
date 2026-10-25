package app

import (
        "context"
        "errors"
        "strings"
        "testing"
        "time"

        "github.com/artemiosu/staffy/internal/action/domain"
        "github.com/artemiosu/staffy/internal/action/port"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.9 step-02: the order status application contour (the TWO
// record commands, the idempotent replay BY KEY, the ladder guard, the
// projection read over the assembled material)
// ---------------------------------------------------------------------------

// fakeOrderFactRepo is the in-memory OrderSupplierFactRepository with
// the store semantics: the per-case fact_no ladder, the UNIQUE full
// fact key (the duplicate append refuses with the honest duplicate
// signal) and the raceHook (the parallel-winner simulation of the
// raced replay tests).
type fakeOrderFactRepo struct {
        facts     []domain.OrderSupplierFact
        nextNumber map[id.UUID]int64
        raceHook  func(repo *fakeOrderFactRepo)
}

func newFakeOrderFactRepo() *fakeOrderFactRepo {
        return &fakeOrderFactRepo{nextNumber: map[id.UUID]int64{}}
}

func (f *fakeOrderFactRepo) factsOf(anchor id.UUID, byCase bool) []domain.OrderSupplierFact {
        out := []domain.OrderSupplierFact{}
        for _, fact := range f.facts {
                if (byCase && fact.CaseID == anchor) || (!byCase && fact.OperationID == anchor) {
                        out = append(out, fact)
                }
        }
        return out
}

func (f *fakeOrderFactRepo) ReadCaseSupplierFacts(_ context.Context, _ tenancy.Context, caseID id.UUID) ([]domain.OrderSupplierFact, error) {
        return f.factsOf(caseID, true), nil
}

func (f *fakeOrderFactRepo) ReadOperationSupplierFacts(_ context.Context, _ tenancy.Context, operationID id.UUID) ([]domain.OrderSupplierFact, error) {
        return f.factsOf(operationID, false), nil
}

func (f *fakeOrderFactRepo) AppendOrderSupplierFact(_ context.Context, p port.AppendOrderSupplierFactParams) (domain.OrderSupplierFact, error) {
        if f.raceHook != nil {
                hook := f.raceHook
                f.raceHook = nil
                hook(f)
        }
        for _, fact := range f.facts {
                if fact.OperationID == p.OperationID && fact.Kind == p.Kind && fact.SupplierOrderID == p.SupplierOrderID {
                        return domain.OrderSupplierFact{}, port.ErrOrderSupplierFactDuplicate
                }
        }
        factNo := f.nextNumber[p.CaseID] + 1
        f.nextNumber[p.CaseID] = factNo
        fact := domain.OrderSupplierFact{
                TenantID: p.Scope.TenantID(), CaseID: p.CaseID, WorkItemID: p.WorkItemID,
                FactNo: factNo, OperationID: p.OperationID, Kind: p.Kind,
                SupplierOrderID: p.SupplierOrderID, HappenedAt: p.HappenedAt,
                ActorID: p.ActorID, RecordedAt: p.Now,
        }
        f.facts = append(f.facts, fact)
        return fact, nil
}

// statusFixture is the full world of the status tests: the committed
// order operation with the APPROVED verification decision (the happy
// ladder of the facts), the wired facts repository and the service.
type statusFixture struct {
        verification *verificationFixture
        facts        *fakeOrderFactRepo
        service      *Service
        operation    domain.Operation
}

// newStatusFixture wires the service over the approved-verification
// world (the unit composition root of the status tests): the
// verification fixture first (the committed operation with the
// receipt), then the APPROVED decision, then the facts wiring.
func newStatusFixture(t *testing.T) *statusFixture {
        t.Helper()
        verification := newVerificationFixture(t)
        if _, err := verification.assign(); err != nil {
                t.Fatalf("назначение проверки обязано пройти: %v", err)
        }
        if _, err := verification.acknowledge(1, 1); err != nil {
                t.Fatalf("принятие проверки обязано пройти: %v", err)
        }
        if _, err := verification.approve(1, 1); err != nil {
                t.Fatalf("подтверждение проверки обязано пройти: %v", err)
        }
        facts := newFakeOrderFactRepo()
        service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
                acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
                WithPreviews(newFakePreviewRepo(), verification.world.control, verification.material, verificationMembers{},
                        fakeEntitlement{status: verification.world.admission}, fakeReadiness{stage: verification.world.stage, instances: verification.world.instances},
                        orderConnectors{state: verification.world.capState, contractVersion: verification.world.contract},
                        fakeRouteFacts{profileVersion: verification.world.profile}, &fakeIDs{}).
                WithAuthorization(newFakeConfirmationRepo(), verification.ops).
                WithDispatch(verification.dispatch, nil).
                WithOrderVerifications(verification.repo).
                WithOrderCorrections(newFakeCorrectionRepo()).
                WithOrderSupplierFacts(facts)
        return &statusFixture{verification: verification, facts: facts, service: service, operation: verification.operation}
}

// formed builds the passing formed record command of the fixture world.
func (f *statusFixture) formed(supplierOrderID string) (OrderSupplierFactOutcome, error) {
        return f.service.RecordOrderSupplierFormed(context.Background(), RecordOrderSupplierFactCommand{
                Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
                CaseID: previewCaseID, OperationID: f.operation.OperationID,
                SupplierOrderID: supplierOrderID, HappenedAt: previewNow.Add(30 * time.Minute), Now: previewNow.Add(31 * time.Minute),
        })
}

// sent builds the passing sent record command of the fixture world.
func (f *statusFixture) sent(supplierOrderID string) (OrderSupplierFactOutcome, error) {
        return f.service.RecordOrderSupplierSent(context.Background(), RecordOrderSupplierFactCommand{
                Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
                CaseID: previewCaseID, OperationID: f.operation.OperationID,
                SupplierOrderID: supplierOrderID, HappenedAt: previewNow.Add(40 * time.Minute), Now: previewNow.Add(41 * time.Minute),
        })
}

// view reads the projection of the fixture world.
func (f *statusFixture) view() (domain.OrderStatusProjection, error) {
        return f.service.ReadOrderStatusView(context.Background(),
                identityDomain.Principal{ID: verificationOwner}, testTenant, previewCaseID, previewNow.Add(time.Hour))
}

// TestRecordOrderSupplierFormedAndSent pins the happy ladder: the
// formed fact lands first (the fact_no ladder), the sent fact of the
// SAME supplier order ID lands second; the two times never mix (the
// named happened_at and the server recorded_at ride the saved facts);
// the outcome source word is the human receipt of the store contour.
func TestRecordOrderSupplierFormedAndSent(t *testing.T) {
        f := newStatusFixture(t)
        formed, err := f.formed("SUP-100")
        if err != nil {
                t.Fatalf("запись формирования обязана пройти: %v", err)
        }
        if formed.Replay {
                t.Fatal("первая запись формирования помечена повтором")
        }
        if formed.Fact.FactNo != 1 {
                t.Fatalf("номер факта %d, а не 1", formed.Fact.FactNo)
        }
        if !formed.Fact.HappenedAt.Equal(previewNow.Add(30 * time.Minute)) {
                t.Fatalf("названное время факта %v", formed.Fact.HappenedAt)
        }
        if !formed.Fact.RecordedAt.Equal(previewNow.Add(31 * time.Minute)) {
                t.Fatalf("серверное время записи %v", formed.Fact.RecordedAt)
        }
        if formed.SourceWord != domain.OrderStatusSupplierManagerSourceWord {
                t.Fatalf("слово источника: %s", formed.SourceWord)
        }
        if formed.Fact.ActorID != verificationManager {
                t.Fatal("актёр факта не менеджер (серверная истина актёра)")
        }
        sent, err := f.sent("SUP-100")
        if err != nil {
                t.Fatalf("запись отправки обязана пройти после формирования: %v", err)
        }
        if sent.Replay || sent.Fact.FactNo != 2 {
                t.Fatalf("запись отправки: replay=%v fact_no=%d", sent.Replay, sent.Fact.FactNo)
        }
}

// TestRecordOrderSupplierFactReplay pins the idempotency BY KEY (AC-9
// — the «webhook repeat» discipline): the repeated formed command of
// the SAME supplier order ID answers the SAVED fact with the replay
// flag — never a second row; the first write wins, the history is
// never rewritten.
func TestRecordOrderSupplierFactReplay(t *testing.T) {
        f := newStatusFixture(t)
        if _, err := f.formed("SUP-100"); err != nil {
                t.Fatalf("первая запись формирования обязана пройти: %v", err)
        }
        replayed, err := f.formed("SUP-100")
        if err != nil {
                t.Fatalf("повтор записи формирования — идемпотентный повтор, а не отказ: %v", err)
        }
        if !replayed.Replay {
                t.Fatal("повтор не помечен replay")
        }
        if replayed.Fact.FactNo != 1 {
                t.Fatalf("повтор вернул факт №%d, а не сохранённый №1", replayed.Fact.FactNo)
        }
        if got := len(f.facts.facts); got != 1 {
                t.Fatalf("фактов %d, а не 1 — вторая строка не должна существовать (AC-9)", got)
        }
        // The formed fact of ANOTHER supplier order ID is a SECOND fact (the
        // multiple suppliers live by name).
        if _, err := f.formed("SUP-200"); err != nil {
                t.Fatalf("формирование другого номера — второй факт, а не отказ: %v", err)
        }
        if got := len(f.facts.facts); got != 2 {
                t.Fatalf("фактов %d, а не 2", got)
        }
}

// TestRecordOrderSupplierFactRacedReplay pins the raced duplicate of
// the store backstop: the UNIQUE full fact key refuses the parallel
// twin — the app re-reads the SAVED fact and answers the honest
// replay outcome (never a masked write).
func TestRecordOrderSupplierFactRacedReplay(t *testing.T) {
        f := newStatusFixture(t)
        // The race: the parallel winner lands INSIDE the append — the
        // duplicate backstop fires on the loser's insert.
        f.facts.raceHook = func(repo *fakeOrderFactRepo) {
                raceScope, _ := tenancy.New(verificationManager, testTenant)
                repo.AppendOrderSupplierFact(context.Background(), port.AppendOrderSupplierFactParams{
                        Scope: raceScope, CaseID: previewCaseID,
                        WorkItemID: f.operation.WorkItemID, OperationID: f.operation.OperationID,
                        Kind: domain.OrderSupplierFactFormed, SupplierOrderID: "SUP-100",
                        HappenedAt: previewNow, ActorID: verificationManager, Now: previewNow,
                })
        }
        outcome, err := f.formed("SUP-100")
        if err != nil {
                t.Fatalf("гонка повтора обязана ответить сохранённым фактом: %v", err)
        }
        if !outcome.Replay || outcome.Fact.FactNo != 1 {
                t.Fatalf("гонка: replay=%v fact_no=%d", outcome.Replay, outcome.Fact.FactNo)
        }
        if got := len(f.facts.facts); got != 1 {
                t.Fatalf("фактов после гонки %d, а не 1", got)
        }
}

// TestRecordOrderSupplierSentLadder pins the honest record ladder: the
// sent fact WITHOUT the prior formed fact of the SAME supplier order
// ID refuses with the ladder word (the discipline of the full history
// — the times are named as they happened, never invented).
func TestRecordOrderSupplierSentLadder(t *testing.T) {
        f := newStatusFixture(t)
        _, err := f.sent("SUP-300")
        if err == nil {
                t.Fatal("отправка без предшествующего формирования прошла — лестница сломана")
        }
        if !strings.Contains(err.Error(), "сначала запишите формирование") {
                t.Fatalf("отказ лестницы не называет слово лестницы: %v", err)
        }
        // The formed fact of ANOTHER ID does not open the sent path.
        if _, err := f.formed("SUP-100"); err != nil {
                t.Fatalf("формирование обязано пройти: %v", err)
        }
        if _, err := f.sent("SUP-300"); err == nil {
                t.Fatal("отправка после формирования ДРУГОГО номера прошла — лестница читает тот же ID")
        }
        if got := len(f.facts.facts); got != 1 {
                t.Fatalf("фактов после отказов лестницы %d, а не 1", got)
        }
}

// TestRecordOrderSupplierFactRefusals pins the honest refusal ladder
// of the record commands: the 404 of a foreign operation anchor (the
// RLS blindness mirror), the 403 of the non-review actor (the scope
// word BEFORE any material) and the 404 of a non-member.
func TestRecordOrderSupplierFactRefusals(t *testing.T) {
        f := newStatusFixture(t)
        // The foreign operation anchor: the operation does not exist in
        // THIS case — the honest 404.
        _, err := f.service.RecordOrderSupplierFormed(context.Background(), RecordOrderSupplierFactCommand{
                Actor: identityDomain.Principal{ID: verificationManager}, TenantID: testTenant,
                CaseID: previewCaseID, OperationID: id.MustParse("01994f64-6c00-7000-8000-0000000000f1"),
                SupplierOrderID: "SUP-100", HappenedAt: previewNow, Now: previewNow,
        })
        if !errors.Is(err, domain.ErrOrderStatusNoOperation) {
                t.Fatalf("чужая операция: %v", err)
        }
        // The non-review actor: the separate no-scope service (the 4-7/4-8
        // precedent — the scope word BEFORE any fact material).
        noScopeService := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{}},
                acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
                WithPreviews(newFakePreviewRepo(), f.verification.world.control, f.verification.material, verificationMembers{},
                        fakeEntitlement{status: f.verification.world.admission}, fakeReadiness{stage: f.verification.world.stage, instances: f.verification.world.instances},
                        orderConnectors{state: f.verification.world.capState, contractVersion: f.verification.world.contract},
                        fakeRouteFacts{profileVersion: f.verification.world.profile}, &fakeIDs{}).
                WithAuthorization(newFakeConfirmationRepo(), f.verification.ops).
                WithDispatch(f.verification.dispatch, nil).
                WithOrderVerifications(f.verification.repo).
                WithOrderCorrections(newFakeCorrectionRepo()).
                WithOrderSupplierFacts(f.facts)
        _, err = noScopeService.RecordOrderSupplierFormed(context.Background(), RecordOrderSupplierFactCommand{
                Actor: identityDomain.Principal{ID: verificationNoScope}, TenantID: testTenant,
                CaseID: previewCaseID, OperationID: f.operation.OperationID,
                SupplierOrderID: "SUP-100", HappenedAt: previewNow, Now: previewNow,
        })
        if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "полномочия review") {
                t.Fatalf("актёр без review: %v", err)
        }
}

// TestReadOrderStatusViewApproved pins the projection read over the
// approved world: the receipt confirms the creation with the system
// receipt BY NAME, the approved decision confirms the verification,
// the estimate material rides the LIVE order composition and the
// timeline carries the receipt + the verification events.
func TestReadOrderStatusViewApproved(t *testing.T) {
        f := newStatusFixture(t)
        projection, err := f.view()
        if err != nil {
                t.Fatalf("чтение проекции обязано пройти: %v", err)
        }
        if !projection.Stages[0].Confirmed || projection.Stages[0].ExternalID != "ABCP-ORDER-77" {
                t.Fatalf("стадия создания: %+v", projection.Stages[0])
        }
        if projection.Stages[0].Source != domain.OrderFactSourceSystemReceipt {
                t.Fatalf("источник стадии создания: %s", projection.Stages[0].Source)
        }
        if !projection.Stages[2].Confirmed {
                t.Fatalf("стадия проверки не подтверждена над approved решением: %+v", projection.Stages[2])
        }
        if projection.CurrentStage != domain.OrderStageManagerVerified {
                t.Fatalf("текущая стадия %s", projection.CurrentStage)
        }
        if !strings.Contains(projection.Deadline.EstimateWord, "не записана") == false && projection.Deadline.EstimateWord == "" {
                t.Fatal("слово оценки пусто")
        }
        if !strings.Contains(projection.NextStep, "сформировать заказ у поставщика") {
                t.Fatalf("следующий шаг: %s", projection.NextStep)
        }
}

// TestReadOrderStatusViewFacts pins the projection read over the full
// fact ladder: the formed and sent facts confirm the supplier stages,
// the supplier system block carries the human facts with the manager
// source and the deadline block records the actual event time.
func TestReadOrderStatusViewFacts(t *testing.T) {
        f := newStatusFixture(t)
        if _, err := f.formed("SUP-100"); err != nil {
                t.Fatalf("формирование обязано пройти: %v", err)
        }
        projection, err := f.view()
        if err != nil {
                t.Fatalf("чтение проекции обязано пройти: %v", err)
        }
        if !projection.Stages[3].Confirmed || projection.Stages[4].Confirmed {
                t.Fatalf("стадии поставщика после одного формирования: formed=%v sent=%v", projection.Stages[3].Confirmed, projection.Stages[4].Confirmed)
        }
        if _, err := f.sent("SUP-100"); err != nil {
                t.Fatalf("отправка обязана пройти: %v", err)
        }
        projection, err = f.view()
        if err != nil {
                t.Fatalf("повторное чтение проекции обязано пройти: %v", err)
        }
        if !projection.Stages[4].Confirmed {
                t.Fatal("стадия отправки не подтверждена при полном покрытии")
        }
        if projection.CurrentStage != domain.OrderStageSupplierSent {
                t.Fatalf("текущая стадия %s", projection.CurrentStage)
        }
        supplier := projection.Systems[2]
        if len(supplier.Facts) != 2 {
                t.Fatalf("фактов поставщика %d, а не 2", len(supplier.Facts))
        }
        for _, fact := range supplier.Facts {
                if fact.Source != domain.OrderFactSourceManagerRecord {
                        t.Fatalf("источник факта поставщика: %s", fact.Source)
                }
        }
        if !projection.Deadline.ActualRecorded {
                t.Fatal("фактическое событие срока не записано при подтверждённой отправке")
        }
}

// TestReadOrderStatusViewBlocked pins the projection read over the
// blocked world (the correction fixture): the blocked decision speaks
// the special state honestly — the correction contour words ride the
// projection, never a hidden «в работе».
func TestReadOrderStatusViewBlocked(t *testing.T) {
        correction := newCorrectionFixture(t)
        facts := newFakeOrderFactRepo()
        service := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
                acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
                WithPreviews(newFakePreviewRepo(), correction.verification.world.control, correction.verification.material, verificationMembers{},
                        fakeEntitlement{status: correction.verification.world.admission}, fakeReadiness{stage: correction.verification.world.stage, instances: correction.verification.world.instances},
                        orderConnectors{state: correction.verification.world.capState, contractVersion: correction.verification.world.contract},
                        fakeRouteFacts{profileVersion: correction.verification.world.profile}, &fakeIDs{}).
                WithAuthorization(correction.confirmations, correction.verification.ops).
                WithDispatch(correction.verification.dispatch, nil).
                WithOrderVerifications(correction.verification.repo).
                WithOrderCorrections(correction.corrections).
                WithOrderSupplierFacts(facts)
        projection, err := service.ReadOrderStatusView(context.Background(),
                identityDomain.Principal{ID: verificationOwner}, testTenant, previewCaseID, previewNow.Add(time.Hour))
        if err != nil {
                t.Fatalf("чтение проекции заблокированного мира обязано пройти: %v", err)
        }
        if projection.SpecialState != domain.OrderStatusBlockedNoCorrectionWord {
                t.Fatalf("спецсостояние заблокированного мира: %s", projection.SpecialState)
        }
        if projection.Stages[2].Confirmed {
                t.Fatal("стадия проверки подтверждена над заблокированным решением")
        }
        // The open correction changes the special state to the correction
        // word (AC-12 — the open correction is NEVER hidden).
        if _, err := correction.open(); err != nil {
                t.Fatalf("открытие коррекции обязано пройти: %v", err)
        }
        projection, err = service.ReadOrderStatusView(context.Background(),
                identityDomain.Principal{ID: verificationOwner}, testTenant, previewCaseID, previewNow.Add(time.Hour))
        if err != nil {
                t.Fatalf("повторное чтение обязано пройти: %v", err)
        }
        if projection.SpecialState != domain.OrderCorrectionPendingWord {
                t.Fatalf("спецсостояние открытой коррекции: %s", projection.SpecialState)
        }
}

// TestReadOrderStatusViewNoOrder pins the honest miss of a case
// WITHOUT an order operation: the projection speaks «операции заказа
// в кейсе нет», the stages stay unconfirmed and NOTHING fails.
func TestReadOrderStatusViewNoOrder(t *testing.T) {
        f := newStatusFixture(t)
        projection, err := f.service.ReadOrderStatusView(context.Background(),
                identityDomain.Principal{ID: verificationOwner}, testTenant,
                id.MustParse("01994f64-6c00-7000-8000-0000000000e9"), previewNow.Add(time.Hour))
        if err != nil {
                t.Fatalf("чтение проекции кейса без заказа обязано пройти честным словом: %v", err)
        }
        if projection.Stages[0].Confirmed {
                t.Fatal("стадия создания подтверждена без операции заказа")
        }
        if !strings.Contains(projection.Stages[0].Word, "операции заказа в кейсе нет") {
                t.Fatalf("слово стадии создания без заказа: %s", projection.Stages[0].Word)
        }
        if projection.CurrentStage != "" {
                t.Fatalf("текущая стадия без заказа: %s", projection.CurrentStage)
        }
}

// TestReadOrderStatusViewRefusals pins the read refusals: the 403 of
// the non-review actor and the unwired-contour word (the honest
// refusal, never a nil panic).
func TestReadOrderStatusViewRefusals(t *testing.T) {
        f := newStatusFixture(t)
        // The non-review actor: the separate no-scope service (the 4-7/4-8
        // precedent).
        noScopeService := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{}},
                acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs()).
                WithPreviews(newFakePreviewRepo(), f.verification.world.control, f.verification.material, verificationMembers{},
                        fakeEntitlement{status: f.verification.world.admission}, fakeReadiness{stage: f.verification.world.stage, instances: f.verification.world.instances},
                        orderConnectors{state: f.verification.world.capState, contractVersion: f.verification.world.contract},
                        fakeRouteFacts{profileVersion: f.verification.world.profile}, &fakeIDs{}).
                WithAuthorization(newFakeConfirmationRepo(), f.verification.ops).
                WithDispatch(f.verification.dispatch, nil).
                WithOrderVerifications(f.verification.repo).
                WithOrderCorrections(newFakeCorrectionRepo()).
                WithOrderSupplierFacts(f.facts)
        _, err := noScopeService.ReadOrderStatusView(context.Background(),
                identityDomain.Principal{ID: verificationNoScope}, testTenant, previewCaseID, previewNow)
        if !errors.Is(err, domain.ErrForbidden) || !strings.Contains(err.Error(), "полномочия review") {
                t.Fatalf("актёр без review: %v", err)
        }
        unwired := New(scopeIdentity{member: true, scopes: map[identityDomain.Scope]bool{identityDomain.Review: true}},
                acceptAllPacks{}, acceptAllCaps{}, orderCatalogDefs())
        _, err = unwired.ReadOrderStatusView(context.Background(),
                identityDomain.Principal{ID: verificationOwner}, testTenant, previewCaseID, previewNow)
        if err == nil || !strings.Contains(err.Error(), "контур статуса заказа не подключён") {
                t.Fatalf("неподключённый контур: %v", err)
        }
}
