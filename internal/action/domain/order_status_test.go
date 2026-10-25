package domain_test

import (
        "reflect"
        "strings"
        "testing"
        "time"

        "github.com/artemiosu/staffy/internal/action/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
)

// Story 4.9 (step-01) — the order status domain tests: the record
// guards (the dictionary, the typed bounds, the sent-after-formed
// ladder), the read-time projection derivations (the full chain, the
// partial send, the AG-4 blocked dispatch, the correction special
// state, the AC-8 divergent IDs, the freshness), the ordered fold
// under the SHUFFLED input (AC-9 — the same projection), the
// byte-stability of the frozen words and the honest probes of the
// slice (the zero supplier-send words, the zero notification fields,
// the zero price fields, the «контур 4-10» boundary).

func statusFixtureIDs() (id.UUID, id.UUID, id.UUID, id.UUID) {
        return id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b901"),
                id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b902"),
                id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b903"),
                id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b904")
}

func statusNow() time.Time {
        return time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
}

func statusReceipt() *domain.OrderStatusReceiptPin {
        return &domain.OrderStatusReceiptPin{
                ReceiptVersionNo: 1,
                ExternalOrderID:  "ABCP-ORDER-99",
                CreatedAt:        time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC),
        }
}

func statusSupplierFact(factNo int64, kind, supplierOrderID string, at time.Time) domain.OrderSupplierFact {
        _, caseID, workItemID, operationID := statusFixtureIDs()
        return domain.OrderSupplierFact{
                TenantID:        id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b900"),
                CaseID:          caseID,
                WorkItemID:      workItemID,
                FactNo:          factNo,
                OperationID:     operationID,
                Kind:            kind,
                SupplierOrderID: supplierOrderID,
                HappenedAt:      at,
                ActorID:         id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b905"),
                RecordedAt:      at.Add(time.Minute),
        }
}

// TestOrderStatusDictionaries pins the compile-time dictionaries of
// the projection: the five lifecycle stages in the lifecycle order, the
// two fact kinds and the two fact sources (AC-3/AC-5).
func TestOrderStatusDictionaries(t *testing.T) {
        if got := strings.Join(domain.OrderStages, ","); got != "client_order_created,pending_manager_verification,manager_verified,supplier_formed,supplier_sent" {
                t.Fatalf("словарь стадий: %v", got)
        }
        if got := strings.Join(domain.OrderSupplierFactKinds, ","); got != "formed,sent" {
                t.Fatalf("словарь видов фактов: %v", got)
        }
        if got := strings.Join(domain.OrderFactSources, ","); got != "system_receipt,manager_record" {
                t.Fatalf("словарь источников фактов: %v", got)
        }
        for _, stage := range domain.OrderStages {
                if domain.OrderStageWords[stage] == "" {
                        t.Fatalf("стадия %s без честного слова", stage)
                }
                if !domain.OrderStageInDictionary(stage) {
                        t.Fatalf("стадия %s вне словаря", stage)
                }
        }
        if domain.OrderStageInDictionary("shipped") {
                t.Fatal("несловарное слово стадии прошло словарь")
        }
        for _, source := range domain.OrderFactSources {
                if domain.OrderFactSourceWords[source] == "" {
                        t.Fatalf("источник %s без честного слова", source)
                }
        }
        if !domain.OrderSupplierFactKindInDictionary("formed") || !domain.OrderSupplierFactKindInDictionary("sent") {
                t.Fatal("словарь видов фактов неполон")
        }
        if domain.OrderSupplierFactKindInDictionary("shipped") {
                t.Fatal("несловарный вид факта прошёл словарь — supplier-send словами фактов не существует")
        }
}

// TestGuardOrderSupplierFactByName pins the honest refusals of the
// record guard: the kind dictionary, the typed supplier-order-ID bound,
// the named happened_at time and the sent-after-formed ladder of the
// SAME supplier order ID.
func TestGuardOrderSupplierFactByName(t *testing.T) {
        happened := statusNow().Add(-2 * time.Hour)
        formed := statusSupplierFact(1, domain.OrderSupplierFactFormed, "SUP-11", happened)
        if err := domain.GuardOrderSupplierFact("shipped", "SUP-11", happened, nil); err == nil {
                t.Fatal("несловарный вид факта прошёл гвард")
        } else if !strings.Contains(err.Error(), "словаря") {
                t.Fatalf("гвард вида не называет словарь: %v", err)
        }
        if err := domain.GuardOrderSupplierFact(domain.OrderSupplierFactFormed, "  ", happened, nil); err == nil {
                t.Fatal("пустой номер заказа поставщика прошёл гвард")
        }
        long := strings.Repeat("S", 201)
        if err := domain.GuardOrderSupplierFact(domain.OrderSupplierFactFormed, long, happened, nil); err == nil {
                t.Fatal("номер заказа поставщика свыше 200 символов прошёл гвард")
        }
        if err := domain.GuardOrderSupplierFact(domain.OrderSupplierFactFormed, "SUP-11", time.Time{}, nil); err == nil {
                t.Fatal("нулевое время факта прошло гвард")
        } else if !strings.Contains(err.Error(), "фактическое время") {
                t.Fatalf("гвард времени не называет два времени: %v", err)
        }
        if err := domain.GuardOrderSupplierFact(domain.OrderSupplierFactSent, "SUP-11", happened, nil); err == nil {
                t.Fatal("отправка без предшествующего формирования прошла гвард")
        } else if !strings.Contains(err.Error(), "сначала запишите формирование") {
                t.Fatalf("гвард лестницы не называет слово лестницы: %v", err)
        }
        other := statusSupplierFact(1, domain.OrderSupplierFactFormed, "SUP-22", happened)
        if err := domain.GuardOrderSupplierFact(domain.OrderSupplierFactSent, "SUP-11", happened, []domain.OrderSupplierFact{other}); err == nil {
                t.Fatal("отправка после формирования ДРУГОГО номера прошла гвард — лестница читает тот же ID")
        }
        if err := domain.GuardOrderSupplierFact(domain.OrderSupplierFactSent, "SUP-11", happened, []domain.OrderSupplierFact{formed}); err != nil {
                t.Fatalf("отправка после формирования того же номера отказала честно: %v", err)
        }
        if err := domain.GuardOrderSupplierFact(domain.OrderSupplierFactFormed, "SUP-11", happened, nil); err != nil {
                t.Fatalf("формирование без предусловий отказало: %v", err)
        }
}

// TestDeriveOrderStatusProjectionFullChain pins the full honest chain:
// the receipt confirms the creation (the system receipt BY NAME), the
// approved decision confirms the verification, the formed and sent
// facts confirm the supplier stages — and NOTHING is ever claimed
// without its evidence.
func TestDeriveOrderStatusProjectionFullChain(t *testing.T) {
        _, _, _, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        approvedAt := statusNow().Add(-1 * time.Hour)
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{
                        Found:           true,
                        OperationID:     operationID,
                        Receipt:         statusReceipt(),
                        OfferEstimate:   "срок оценки 2-3 дня",
                },
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: domain.OrderVerification{
                                VerificationNo:  1,
                                OperationID:     operationID,
                                ExternalOrderID: "ABCP-ORDER-99",
                        },
                        Events: []domain.OrderVerificationEvent{
                                {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventAssigned, Assignee: assignee, ActorID: assignee, CreatedAt: statusNow().Add(-3 * time.Hour)},
                                {VerificationNo: 1, EventNo: 2, Kind: domain.OrderVerificationEventAcknowledged, ActorID: assignee, CreatedAt: statusNow().Add(-2 * time.Hour)},
                                {VerificationNo: 1, EventNo: 3, Kind: domain.OrderVerificationEventApproved, ExternalOrderID: "ABCP-ORDER-99", ActorID: assignee, CreatedAt: approvedAt},
                        },
                }},
                Facts: []domain.OrderSupplierFact{
                        statusSupplierFact(1, domain.OrderSupplierFactFormed, "SUP-11", statusNow().Add(-40*time.Minute)),
                        statusSupplierFact(2, domain.OrderSupplierFactSent, "SUP-11", statusNow().Add(-30*time.Minute)),
                },
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        if len(projection.Stages) != 5 {
                t.Fatalf("стадий %d, а не 5", len(projection.Stages))
        }
        for _, stage := range projection.Stages {
                switch stage.Stage {
                case domain.OrderStageClientOrderCreated, domain.OrderStageManagerVerified, domain.OrderStageSupplierFormed, domain.OrderStageSupplierSent:
                        if !stage.Confirmed {
                                t.Fatalf("стадия %s не подтверждена в полной цепочке", stage.Stage)
                        }
                case domain.OrderStagePendingManagerVerification:
                        // The honest current-state read: the task is CLOSED (the
                        // approved decision) — the pending stage is a MOMENT of the
                        // history, not a permanent rung (the history rides the
                        // timeline, AC-9).
                        if stage.Confirmed {
                                t.Fatal("стадия ожидания проверки подтверждена при закрытой задаче")
                        }
                }
        }
        if projection.Stages[0].Source != domain.OrderFactSourceSystemReceipt {
                t.Fatalf("источник стадии создания %s, а не system_receipt", projection.Stages[0].Source)
        }
        if projection.Stages[0].ExternalID != "ABCP-ORDER-99" {
                t.Fatalf("внешний ID стадии создания %s", projection.Stages[0].ExternalID)
        }
        if projection.Stages[2].ExternalID != "ABCP-ORDER-99" {
                t.Fatalf("внешний ID стадии проверки %s", projection.Stages[2].ExternalID)
        }
        if projection.CurrentStage != domain.OrderStageSupplierSent {
                t.Fatalf("текущая стадия %s, а не supplier_sent", projection.CurrentStage)
        }
        if projection.SpecialState != "" {
                t.Fatalf("спецсостояние в полной цепочке непусто: %s", projection.SpecialState)
        }
        if !projection.Deadline.ActualRecorded {
                t.Fatal("фактическое событие срока не записано при подтверждённой отправке")
        }
        if !strings.Contains(projection.Deadline.EstimateWord, "2-3 дня") {
                t.Fatalf("оценка срока не несёт материал 4-6: %s", projection.Deadline.EstimateWord)
        }
        if !strings.Contains(projection.NextStep, "ожидание поставки") {
                t.Fatalf("следующий шаг полной цепочки: %s", projection.NextStep)
        }
        // The systems: ABCP carries the receipt fact; 1C stays unknown; the
        // supplier carries both human facts.
        if len(projection.Systems) != 3 {
                t.Fatalf("систем %d, а не 3", len(projection.Systems))
        }
        if projection.Systems[0].System != domain.OrderStatusSystemABCP || !strings.Contains(projection.Systems[0].Word, "квитанция") {
                t.Fatalf("слово ABCP: %s", projection.Systems[0].Word)
        }
        if !strings.Contains(projection.Systems[1].Word, "unknown") {
                t.Fatalf("слово 1С: %s", projection.Systems[1].Word)
        }
        if len(projection.Systems[2].Facts) != 2 {
                t.Fatalf("фактов поставщика %d, а не 2", len(projection.Systems[2].Facts))
        }
        for _, fact := range projection.Systems[2].Facts {
                if fact.Source != domain.OrderFactSourceManagerRecord {
                        t.Fatalf("источник факта поставщика %s, а не manager_record", fact.Source)
                }
        }
        // The timeline carries the receipt, the three verification events
        // and the two supplier facts.
        if len(projection.Timeline) != 6 {
                t.Fatalf("лента переходов %d рядов, а не 6", len(projection.Timeline))
        }
        if projection.Timeline[0].Source != domain.OrderFactSourceSystemReceipt {
                t.Fatalf("первый ряд ленты не квитанция: %+v", projection.Timeline[0])
        }
}

// TestDeriveOrderStatusProjectionPending pins the honest early state:
// the receipt + the ACTIVE verification task confirm the first two
// stages; the supplier stages stay unconfirmed with their honest
// «не утверждается» words; the client words carry the boundary word.
func TestDeriveOrderStatusProjectionPending(t *testing.T) {
        _, _, _, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{
                        Found:       true,
                        OperationID: operationID,
                        Receipt:     statusReceipt(),
                },
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: domain.OrderVerification{VerificationNo: 1, OperationID: operationID},
                        Events: []domain.OrderVerificationEvent{
                                {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventAssigned,
                                        Assignee: assignee, VerifyDeadline: domain.Verify4h,
                                        DeadlineAt: statusNow().Add(2 * time.Hour), ActorID: assignee,
                                        CreatedAt: statusNow().Add(-1 * time.Hour)},
                        },
                }},
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        if !projection.Stages[0].Confirmed || !projection.Stages[1].Confirmed {
                t.Fatal("стадии создания/проверки не подтверждены в раннем состоянии")
        }
        if projection.Stages[2].Confirmed || projection.Stages[3].Confirmed || projection.Stages[4].Confirmed {
                t.Fatal("стадии выше проверки подтверждены без доказательств")
        }
        if !strings.Contains(projection.Stages[4].Word, "утверждение об отправке не делается") {
                t.Fatalf("слово неподтверждённой отправки: %s", projection.Stages[4].Word)
        }
        if projection.CurrentStage != domain.OrderStagePendingManagerVerification {
                t.Fatalf("текущая стадия %s", projection.CurrentStage)
        }
        if !strings.Contains(projection.Stages[1].Word, "назначен") {
                t.Fatalf("слово стадии проверки не несёт состояние задачи: %s", projection.Stages[1].Word)
        }
        if !strings.Contains(projection.NextStep, "проверить заказ менеджером") {
                t.Fatalf("следующий шаг раннего состояния: %s", projection.NextStep)
        }
        clientJoined := strings.Join(projection.ClientWords, " ")
        if !strings.Contains(clientJoined, domain.OrderClientBoundaryWord) {
                t.Fatalf("клиентские слова без границы: %s", clientJoined)
        }
        if projection.Systems[2].Word != domain.OrderStatusSupplierNotRecordedWord {
                t.Fatalf("слово поставщика без фактов: %s", projection.Systems[2].Word)
        }
}

// TestDeriveOrderStatusProjectionBlockedNoCorrection pins the blocked
// special state: the last closed blocked decision speaks the honest
// owner's-call word; the verified stage reads the LADDER DOWN (an older
// approved decision above a NEW blocked one does not survive).
func TestDeriveOrderStatusProjectionBlockedNoCorrection(t *testing.T) {
        _, _, _, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt()},
                Verifications: []domain.OrderStatusVerificationMaterial{
                        {
                                Task: domain.OrderVerification{VerificationNo: 1, OperationID: operationID},
                                Events: []domain.OrderVerificationEvent{
                                        {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventApproved, ActorID: assignee, CreatedAt: statusNow().Add(-4 * time.Hour)},
                                },
                        },
                        {
                                Task: domain.OrderVerification{VerificationNo: 2, OperationID: operationID},
                                Events: []domain.OrderVerificationEvent{
                                        {VerificationNo: 2, EventNo: 1, Kind: domain.OrderVerificationEventAssigned, Assignee: assignee, ActorID: assignee, CreatedAt: statusNow().Add(-3 * time.Hour)},
                                        {VerificationNo: 2, EventNo: 2, Kind: domain.OrderVerificationEventBlocked, Positions: []string{"WF2B196K666AA"}, Reason: domain.VerifyBlockReasonWrongPart, ActorID: assignee, CreatedAt: statusNow().Add(-2 * time.Hour)},
                                },
                        },
                },
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        if projection.Stages[2].Confirmed {
                t.Fatal("стадия проверки подтверждена над более новым заблокированным решением — лестница вниз читает историю")
        }
        if projection.SpecialState != domain.OrderStatusBlockedNoCorrectionWord {
                t.Fatalf("спецсостояние блокировки: %s", projection.SpecialState)
        }
        if !strings.Contains(projection.NextStep, "решение за владельцем") {
                t.Fatalf("следующий шаг блокировки: %s", projection.NextStep)
        }
}

// TestDeriveOrderStatusProjectionOpenCorrection pins the AC-12 special
// state: the open correction is NEVER hidden behind «в работе» — the
// word rides the projection with the owner and the deadline of the
// live 4-8 derivation.
func TestDeriveOrderStatusProjectionOpenCorrection(t *testing.T) {
        _, caseID, workItemID, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        owner := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b907")
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt()},
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: domain.OrderVerification{VerificationNo: 1, OperationID: operationID},
                        Events: []domain.OrderVerificationEvent{
                                {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventAssigned, Assignee: assignee, ActorID: assignee, CreatedAt: statusNow().Add(-3 * time.Hour)},
                                {VerificationNo: 1, EventNo: 2, Kind: domain.OrderVerificationEventBlocked, ActorID: assignee, CreatedAt: statusNow().Add(-2 * time.Hour)},
                        },
                }},
                Corrections: []domain.OrderStatusCorrectionMaterial{{
                        Correction: domain.OrderCorrection{
                                CaseID: caseID, WorkItemID: workItemID, CorrectionNo: 1, OperationID: operationID,
                                VerificationNo: 1, ReceiptVersionNo: 1, ExternalOrderID: "ABCP-ORDER-99",
                        },
                        Events: []domain.OrderCorrectionEvent{
                                {CaseID: caseID, CorrectionNo: 1, EventNo: 1, Kind: domain.OrderCorrectionEventOpened,
                                        Owner: owner, CorrectDeadline: domain.Correct4h, DeadlineAt: statusNow().Add(2 * time.Hour),
                                        Positions: []string{"WF2B196K666AA"}, Reason: domain.VerifyBlockReasonWrongPart,
                                        ActorID: owner, CreatedAt: statusNow().Add(-1 * time.Hour)},
                        },
                }},
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        if projection.SpecialState != domain.OrderCorrectionPendingWord {
                t.Fatalf("спецсостояние открытой коррекции: %s", projection.SpecialState)
        }
        if projection.SpecialOwner != owner {
                t.Fatal("владелец спецсостояния не владелец коррекции")
        }
        if !strings.Contains(projection.SpecialDetail, "дедлайн коррекции") {
                t.Fatalf("деталь спецсостояния без дедлайна: %s", projection.SpecialDetail)
        }
        if !strings.Contains(projection.NextStep, "перечит") {
                t.Fatalf("следующий шаг открытой коррекции: %s", projection.NextStep)
        }
}

// TestDeriveOrderStatusProjectionPartialSend pins the AC-15 honest
// aggregate: the formed facts of TWO suppliers with only ONE sent
// record — the stage supplier_sent stays UNCONFIRMED, every formation
// is named, the «отправлен частично» word rides the stage.
func TestDeriveOrderStatusProjectionPartialSend(t *testing.T) {
        _, _, _, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt()},
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: domain.OrderVerification{VerificationNo: 1, OperationID: operationID},
                        Events: []domain.OrderVerificationEvent{
                                {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventApproved, ActorID: assignee, CreatedAt: statusNow().Add(-1 * time.Hour)},
                        },
                }},
                Facts: []domain.OrderSupplierFact{
                        statusSupplierFact(1, domain.OrderSupplierFactFormed, "SUP-11", statusNow().Add(-40*time.Minute)),
                        statusSupplierFact(2, domain.OrderSupplierFactFormed, "SUP-22", statusNow().Add(-35*time.Minute)),
                        statusSupplierFact(3, domain.OrderSupplierFactSent, "SUP-11", statusNow().Add(-30*time.Minute)),
                },
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        if projection.Stages[3].Confirmed != true {
                t.Fatal("стадия формирования не подтверждена при двух записях")
        }
        if projection.Stages[4].Confirmed {
                t.Fatal("стадия отправки подтверждена при частичной отправке — ложное утверждение об отправке")
        }
        if !strings.Contains(projection.Stages[4].Word, "отправлен частично") {
                t.Fatalf("слово частичной отправки: %s", projection.Stages[4].Word)
        }
        if !strings.Contains(projection.Stages[4].Detail, "SUP-11 отправлен") {
                t.Fatalf("деталь частичной отправки без покрытия: %s", projection.Stages[4].Detail)
        }
        if !strings.Contains(projection.Stages[4].Detail, "SUP-22 сформирован и не записан как отправленный") {
                t.Fatalf("деталь частичной отправки без поимённого непокрытия: %s", projection.Stages[4].Detail)
        }
        if projection.CurrentStage != domain.OrderStageSupplierFormed {
                t.Fatalf("текущая стадия частичной отправки %s", projection.CurrentStage)
        }
        if !strings.Contains(projection.NextStep, "SUP-22") {
                t.Fatalf("следующий шаг частичной отправки без поимённого номера: %s", projection.NextStep)
        }
}

// TestDeriveOrderStatusProjectionAG4Blocked pins the honest AG-4
// production contour: the dispatch_blocked operation with NO receipt —
// the creation stage stays unconfirmed BY NAME with the manual-contour
// word; the ABCP system block speaks the manual contour; the next step
// names the manual creation.
func TestDeriveOrderStatusProjectionAG4Blocked(t *testing.T) {
        _, _, _, operationID := statusFixtureIDs()
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{
                        Found:           true,
                        OperationID:     operationID,
                        DispatchBlocked: true,
                },
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        if projection.Stages[0].Confirmed {
                t.Fatal("стадия создания подтверждена без квитанции при dispatch_blocked")
        }
        if !strings.Contains(projection.Stages[0].Word, "вручную в контуре магазина") {
                t.Fatalf("слово стадии при AG-4: %s", projection.Stages[0].Word)
        }
        if !strings.Contains(projection.Systems[0].Word, "AG-4") {
                t.Fatalf("слово системы ABCP при AG-4: %s", projection.Systems[0].Word)
        }
        if !strings.Contains(projection.NextStep, "создать заказ в ABCP вручную") {
                t.Fatalf("следующий шаг при AG-4: %s", projection.NextStep)
        }
        if projection.CurrentStage != "" {
                t.Fatalf("текущая стадия при AG-4 непуста: %s", projection.CurrentStage)
        }
}

// TestDeriveOrderStatusProjectionIDDivergence pins the AC-8 honesty:
// the receipt external ID and the reread external ID diverge — BOTH
// facts ride the ABCP block with their OWN sources, the divergence
// word names the refusal of the favorable choice.
func TestDeriveOrderStatusProjectionIDDivergence(t *testing.T) {
        _, caseID, workItemID, operationID := statusFixtureIDs()
        owner := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b907")
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt()},
                Corrections: []domain.OrderStatusCorrectionMaterial{{
                        Correction: domain.OrderCorrection{
                                CaseID: caseID, WorkItemID: workItemID, CorrectionNo: 1, OperationID: operationID,
                                VerificationNo: 1, ReceiptVersionNo: 1, ExternalOrderID: "ABCP-ORDER-99",
                        },
                        Events: []domain.OrderCorrectionEvent{
                                {CaseID: caseID, CorrectionNo: 1, EventNo: 1, Kind: domain.OrderCorrectionEventOpened,
                                        Owner: owner, CorrectDeadline: domain.Correct4h, DeadlineAt: statusNow().Add(4 * time.Hour),
                                        Positions: []string{"WF2B196K666AA"}, Reason: domain.VerifyBlockReasonWrongPart,
                                        ActorID: owner, CreatedAt: statusNow().Add(-2 * time.Hour)},
                                {CaseID: caseID, CorrectionNo: 1, EventNo: 2, Kind: domain.OrderCorrectionEventReread,
                                        ExternalObjectID: "ABCP-ORDER-100", ActorID: owner, CreatedAt: statusNow().Add(-1 * time.Hour)},
                        },
                }},
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        abcp := projection.Systems[0]
        if !abcp.DivergentIDs {
                t.Fatal("флаг расхождения внешних ID не выставлен")
        }
        if len(abcp.Facts) != 2 {
                t.Fatalf("фактов ABCP %d, а не 2 (оба раздельно)", len(abcp.Facts))
        }
        sources := map[string]string{}
        for _, fact := range abcp.Facts {
                sources[fact.ExternalID] = fact.Source
        }
        if sources["ABCP-ORDER-99"] != domain.OrderFactSourceSystemReceipt {
                t.Fatalf("источник квитанционного ID: %s", sources["ABCP-ORDER-99"])
        }
        if sources["ABCP-ORDER-100"] != domain.OrderFactSourceManagerRecord {
                t.Fatalf("источник перечитанного ID: %s", sources["ABCP-ORDER-100"])
        }
        if !strings.Contains(abcp.Word, domain.OrderStatusExternalIDDivergenceWord) {
                t.Fatalf("слово ABCP без слова расхождения: %s", abcp.Word)
        }
}

// TestDeriveOrderStatusProjectionShuffle pins the AC-9 fold: the SAME
// material with the SHUFFLED slices (the verifications, the events,
// the corrections, the facts) yields the IDENTICAL projection — the
// old event never rolls the new one back, the fold reads the monotonic
// anchors, never the arrival order.
func TestDeriveOrderStatusProjectionShuffle(t *testing.T) {
        _, caseID, workItemID, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        owner := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b907")
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt(), OfferEstimate: "срок 2 дня"},
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: domain.OrderVerification{VerificationNo: 1, OperationID: operationID},
                        Events: []domain.OrderVerificationEvent{
                                {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventAssigned, Assignee: assignee, VerifyDeadline: domain.Verify1d, DeadlineAt: statusNow().Add(20 * time.Hour), ActorID: assignee, CreatedAt: statusNow().Add(-5 * time.Hour)},
                                {VerificationNo: 1, EventNo: 2, Kind: domain.OrderVerificationEventAcknowledged, ActorID: assignee, CreatedAt: statusNow().Add(-4 * time.Hour)},
                                {VerificationNo: 1, EventNo: 3, Kind: domain.OrderVerificationEventApproved, ExternalOrderID: "ABCP-ORDER-99", ActorID: assignee, CreatedAt: statusNow().Add(-3 * time.Hour)},
                        },
                }},
                Corrections: []domain.OrderStatusCorrectionMaterial{{
                        Correction: domain.OrderCorrection{CaseID: caseID, WorkItemID: workItemID, CorrectionNo: 1, OperationID: operationID, VerificationNo: 1, ReceiptVersionNo: 1, ExternalOrderID: "ABCP-ORDER-99"},
                        Events: []domain.OrderCorrectionEvent{
                                {CaseID: caseID, CorrectionNo: 1, EventNo: 1, Kind: domain.OrderCorrectionEventOpened, Owner: owner, CorrectDeadline: domain.Correct4h, DeadlineAt: statusNow().Add(2 * time.Hour), Positions: []string{"WF2B196K666AA"}, Reason: domain.VerifyBlockReasonWrongPart, ActorID: owner, CreatedAt: statusNow().Add(-2 * time.Hour)},
                        },
                }},
                Facts: []domain.OrderSupplierFact{
                        statusSupplierFact(1, domain.OrderSupplierFactFormed, "SUP-11", statusNow().Add(-90*time.Minute)),
                        statusSupplierFact(2, domain.OrderSupplierFactSent, "SUP-11", statusNow().Add(-80*time.Minute)),
                },
        }
        baseline := domain.DeriveOrderStatusProjection(material, statusNow())
        // The shuffle: every slice reversed (the events, the tasks, the
        // corrections, the facts) — the arrival order is the WORST case of
        // the read order.
        shuffled := domain.OrderStatusMaterial{
                Operation: material.Operation,
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: material.Verifications[0].Task,
                        Events: []domain.OrderVerificationEvent{
                                material.Verifications[0].Events[2],
                                material.Verifications[0].Events[0],
                                material.Verifications[0].Events[1],
                        },
                }},
                Corrections: []domain.OrderStatusCorrectionMaterial{{
                        Correction: material.Corrections[0].Correction,
                        Events:     []domain.OrderCorrectionEvent{material.Corrections[0].Events[0]},
                }},
                Facts: []domain.OrderSupplierFact{material.Facts[1], material.Facts[0]},
        }
        shuffledProjection := domain.DeriveOrderStatusProjection(shuffled, statusNow())
        if !reflect.DeepEqual(baseline.Stages, shuffledProjection.Stages) {
                t.Fatalf("стадии изменились при перемешанном входе (AC-9):\n%+v\n%+v", baseline.Stages, shuffledProjection.Stages)
        }
        if !reflect.DeepEqual(baseline.Timeline, shuffledProjection.Timeline) {
                t.Fatalf("лента изменилась при перемешанном входе (AC-9):\n%+v\n%+v", baseline.Timeline, shuffledProjection.Timeline)
        }
        if !reflect.DeepEqual(baseline.Systems, shuffledProjection.Systems) {
                t.Fatal("системные блоки изменились при перемешанном входе (AC-9)")
        }
        if baseline.CurrentStage != shuffledProjection.CurrentStage || baseline.NextStep != shuffledProjection.NextStep {
                t.Fatal("текущая стадия/следующий шаг изменились при перемешанном входе (AC-9)")
        }
}

// TestDeriveOrderStatusTimelineFold pins the ordered fold of the
// transitions: the receipt first, the verification events by the
// monotonic event number, the supplier facts by the named happened_at
// display time with the stable anchor tie.
func TestDeriveOrderStatusTimelineFold(t *testing.T) {
        _, _, _, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        same := statusNow().Add(-30 * time.Minute)
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt()},
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: domain.OrderVerification{VerificationNo: 1, OperationID: operationID},
                        Events: []domain.OrderVerificationEvent{
                                {VerificationNo: 1, EventNo: 2, Kind: domain.OrderVerificationEventAcknowledged, ActorID: assignee, CreatedAt: statusNow().Add(-2 * time.Hour)},
                                {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventAssigned, Assignee: assignee, ActorID: assignee, CreatedAt: statusNow().Add(-3 * time.Hour)},
                        },
                }},
                Facts: []domain.OrderSupplierFact{statusSupplierFact(1, domain.OrderSupplierFactFormed, "SUP-11", same)},
        }
        timeline := domain.DeriveOrderStatusTimeline(material)
        if len(timeline) != 4 {
                t.Fatalf("лента %d рядов, а не 4", len(timeline))
        }
        if timeline[0].Machine != "order" || timeline[0].Kind != "receipt_committed" {
                t.Fatalf("первый ряд ленты не квитанция: %+v", timeline[0])
        }
        if timeline[1].Machine != "verification" || timeline[1].Kind != "verification_assigned" {
                t.Fatalf("второй ряд ленты не назначение (порядок событий сломан): %+v", timeline[1])
        }
        if timeline[3].Machine != "supplier_fact" {
                t.Fatalf("четвёртый ряд ленты не факт поставщика: %+v", timeline[3])
        }
}

// TestOrderStatusWordsByteStability pins the frozen honest words of
// the slice (the byte-exact anchors the panels and the tests read).
func TestOrderStatusWordsByteStability(t *testing.T) {
        pins := map[string]string{
                domain.OrderStatusManualContourWord:        "заказ создаётся вручную в контуре магазина (AG-4, рекомендация order_creation 4-6)",
                domain.OrderStatusSentAfterFormedWord:      "отправление записывается после записи формирования того же заказа поставщику — сначала запишите формирование (дисциплина истории, время формирования назовите фактическим)",
                domain.OrderStatusPartialSentWord:          "отправлен частично: не все записанные формирования покрыты записями отправки — стадия «отправлен» не подтверждается",
                domain.OrderStatusSupplierNotRecordedWord:  "не записано: фактов формирования/отправки поставщику нет (Staffy не читает внешние системы — AG-4)",
                domain.OrderStatusExternalIDDivergenceWord: "внешние идентификаторы расходятся: показаны оба значения раздельно со своими источниками — благоприятная версия не выбирается",
                domain.OrderStatusFreshnessWord:            "живая деривация при открытии панели; внешние чтения ABCP/1С/поставщика не подключены (AG-4) — воркеров и расписания нет",
                domain.OrderStatusNoNotificationsWord:      "уведомления о переходах не отправляются: канал не подключён (OQ-1/OQ-2); панель — единственная поверхность состояния; доставка и прочтение не фиксируются",
                domain.OrderStatusDeadlineConfirmedWord:    "подтверждённая дата — не существует: подтверждения поставщиком нет",
        }
        for word, expected := range pins {
                if word != expected {
                        t.Fatalf("слово изменено:\n есть: %s\n ждали: %s", word, expected)
                }
        }
}

// TestOrderStatusHonestProbes pins the structural honesty of the
// slice: NO supplier-send word exists in the kind dictionary, NO
// notification field exists on the projection, NO price field exists
// on the projection/facts (the purchasing data stays hidden), and the
// projection words carry the boundary words (4-10/2-13/3-8) and the
// 1C unknown (OQ-4).
func TestOrderStatusHonestProbes(t *testing.T) {
        for _, kind := range domain.OrderSupplierFactKinds {
                if strings.Contains(kind, "send") || strings.Contains(kind, "ship") {
                        t.Fatalf("вид факта %s выглядит словом отправки — факты записывают реальность, не выполняют её", kind)
                }
        }
        // The kind words are the honest claims of the recorded reality —
        // never the send commands (the OQ-3 fence probe rides the transport
        // layer too; here the words stay «запись менеджера»).
        for _, kind := range domain.OrderSupplierFactKinds {
                if !strings.Contains(domain.OrderSupplierFactKindWords[kind], "запись менеджера") {
                        t.Fatalf("слово вида факта %s не запись менеджера: %s", kind, domain.OrderSupplierFactKindWords[kind])
                }
        }
        // The projection carries the boundary words — never a hidden state.
        _, _, _, operationID := statusFixtureIDs()
        projection := domain.DeriveOrderStatusProjection(domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt()},
        }, statusNow())
        if !strings.Contains(projection.NotificationWord, "не отправляются") {
                t.Fatalf("слово уведомлений: %s", projection.NotificationWord)
        }
        if !strings.Contains(projection.DisputeWord, "не переписывает историю") {
                t.Fatalf("слово оспаривания: %s", projection.DisputeWord)
        }
        if !strings.Contains(projection.CancellationWord, "4-10") {
                t.Fatalf("слово-граница отмены: %s", projection.CancellationWord)
        }
        if !strings.Contains(projection.Freshness.NoScheduleWord, "расписание не существует") {
                t.Fatalf("слово расписания: %s", projection.Freshness.NoScheduleWord)
        }
        if !strings.Contains(projection.Systems[1].Word, "OQ-4") {
                t.Fatalf("слово 1С без пина OQ-4: %s", projection.Systems[1].Word)
        }
        if !strings.Contains(projection.Deadline.ConfirmedWord, "не существует") {
                t.Fatalf("слово подтверждённой даты: %s", projection.Deadline.ConfirmedWord)
        }
        // The freshness carries the last confirmed fact time (the receipt
        // time of the fixture).
        if !projection.Freshness.LastConfirmedAt.Equal(statusReceipt().CreatedAt) {
                t.Fatalf("время последнего факта %v", projection.Freshness.LastConfirmedAt)
        }
}

// TestOrderStatusFreshnessCarriesFacts pins the AC-10 fold of the last
// confirmed fact: the max over the receipt, the events and the facts
// (BOTH times of every fact — the happened and the recorded).
func TestOrderStatusFreshnessCarriesFacts(t *testing.T) {
        _, _, _, operationID := statusFixtureIDs()
        assignee := id.UUID("0f6d92b6-1f9e-7b8a-9a11-76e64ba4b906")
        happened := statusNow().Add(-10 * time.Minute)
        fact := statusSupplierFact(1, domain.OrderSupplierFactFormed, "SUP-11", happened)
        material := domain.OrderStatusMaterial{
                Operation: domain.OrderStatusOperationMaterial{Found: true, OperationID: operationID, Receipt: statusReceipt()},
                Verifications: []domain.OrderStatusVerificationMaterial{{
                        Task: domain.OrderVerification{VerificationNo: 1, OperationID: operationID},
                        Events: []domain.OrderVerificationEvent{
                                {VerificationNo: 1, EventNo: 1, Kind: domain.OrderVerificationEventAssigned, Assignee: assignee, ActorID: assignee, CreatedAt: statusNow().Add(-1 * time.Hour)},
                        },
                }},
                Facts: []domain.OrderSupplierFact{fact},
        }
        projection := domain.DeriveOrderStatusProjection(material, statusNow())
        if !projection.Freshness.LastConfirmedAt.Equal(fact.RecordedAt) {
                t.Fatalf("время последнего факта %v, а не recorded_at факта %v", projection.Freshness.LastConfirmedAt, fact.RecordedAt)
        }
}
