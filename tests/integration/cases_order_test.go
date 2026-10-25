//go:build integration

package integration_test

import (
        "context"
        "errors"
        "fmt"
        "strings"
        "testing"
        "time"

        actionAPI "github.com/artemiosu/staffy/internal/action/api"
        actionApp "github.com/artemiosu/staffy/internal/action/app"
        actionCatalog "github.com/artemiosu/staffy/internal/action/catalog"
        actionDomain "github.com/artemiosu/staffy/internal/action/domain"
        actionPort "github.com/artemiosu/staffy/internal/action/port"
        actionStore "github.com/artemiosu/staffy/internal/action/store"
        "github.com/artemiosu/staffy/internal/adapters/actiondispatcher"
        "github.com/artemiosu/staffy/internal/adapters/capabilityresolver"
        "github.com/artemiosu/staffy/internal/adapters/controleffectiveresolver"
        "github.com/artemiosu/staffy/internal/adapters/owningsystem"
        "github.com/artemiosu/staffy/internal/adapters/packresolver"
        "github.com/artemiosu/staffy/internal/adapters/reconciliationsyn"
        casesAPI "github.com/artemiosu/staffy/internal/cases/api"
        casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
        connectorAPI "github.com/artemiosu/staffy/internal/connector/api"
        connectorDomain "github.com/artemiosu/staffy/internal/connector/domain"
        controlStore "github.com/artemiosu/staffy/internal/control/store"
        identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.6 (step-04): the LIVE order eval matrix — the full chain
// (selection → builder proposal → preview → confirmation → authorization →
// synthetic dispatch → receipt with the derived words) and the AC-15 rows
// by name over the REAL PG stores.
// ---------------------------------------------------------------------------

// orderNow is the clock of the order matrix (one hour after the dispatch
// matrix — the confirmation windows pin around this moment).
var orderNow = actionDispatchNow.Add(1 * time.Hour)

// orderConnectorFacts is the TEST-CONTOUR capability source of the order
// world: the abcp-parts connection with command.prepare-client-order in
// the VERIFIED state (the synthetic truth of the test circuit — the
// production contour honestly reads the mutating capability as unverified
// with the Action Gateway block, AG-4, and the real dispatch stays behind
// the nil adapter; the class of the synthetic mutation adapter of 3-6).
type orderConnectorFacts struct{}

func (orderConnectorFacts) ConnectionsFacts(_ context.Context, _ tenancy.Context) ([]connectorAPI.ConnectionFacts, error) {
        return []connectorAPI.ConnectionFacts{{
                ConnectionID:    id.MustParse("01994f64-6c00-7000-8000-00000000ca01"),
                ConnectorID:     "abcp-parts",
                ContractVersion: "1.1.0",
                Capabilities: []connectorDomain.CapabilityReport{{
                        CapabilityDeclaration: connectorDomain.CapabilityDeclaration{ID: "command.prepare-client-order"},
                        State:                 connectorDomain.CapStateVerified,
                }},
        }}, nil
}

// liveOrderVersionSource is the LIVE order version source of the test
// contour: the derivation re-runs over the REAL case view (the same
// DeriveOrderCommitmentMaterial the builder used — the honest
// revalidation reads the live truth, never a stored copy).
type liveOrderVersionSource struct {
        cases  casesAPI.Service
        actor  identityDomain.Principal
        tenant id.UUID
}

func (s liveOrderVersionSource) OrderCommitmentDigest(ctx context.Context, request actionPort.OrderVersionRequest) (string, error) {
        view, err := s.cases.CaseView(ctx, s.actor, s.tenant, request.CaseID)
        if err != nil {
                return "", err
        }
        material, err := actionApp.DeriveOrderCommitmentMaterial(view)
        if err != nil {
                return "", err
        }
        return actionDomain.OrderDigestOf(material), nil
}

// orderWorld builds the matrix world: the cases world of the 4-5 chain
// plus the REAL action contour over the PRODUCT dictionary (the first
// product entry create_abcp_customer_order@1.0.0) with the synthetic
// mutation adapter serving command.prepare-client-order, the synthetic
// fresh reader and the LIVE order version source.
func orderWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, *owningsystem.Synthetic, *reconciliationsyn.Synthetic, identityDomain.Principal) {
        t.Helper()
        w := newCasesWorld(t, seed)
        w.cases = w.buildCasesServiceClock(actionPreviewIDs(seed*10+500, 512), fixedClock{operationNow})
        // The pack admission chain of the world (ONCE — the 6.0.0 pin with the
        // comparison policy; every row case inherits the admitted employee).
        offersSealFullChain(t, w, w.employeeID)
        reader := owningsystem.New()
        adapter := &countingMutationAdapter{inner: actiondispatcher.NewServing("command.prepare-client-order")}
        reconcile := reconciliationsyn.NewServing("command.prepare-client-order")
        // The acting operator-manager of the pilot: the store owner (the
        // step-up principal of the work surfaces — the owner role carries the
        // review scope of the preparation surface, the operations scope of the
        // drain command and the operator word of the definition's allowed
        // roles; the bare operator role of the identity grant matrix carries
        // no review scope — the pilot's operating manager IS the owner).
        operator := w.workActor()
        source := liveOrderVersionSource{cases: w.cases, actor: operator, tenant: w.tenant}
        service := w.buildOrderService(adapter, reader, reconcile, source, actionPreviewIDs(seed*10+400, 512))
        return w, service, adapter, reader, reconcile, operator
}

// buildOrderService wires the REAL app service over the PRODUCT action
// dictionary (the first product entry — the admission walk at
// construction is the honest green over the live resolvers), the
// synthetic capability facts of the order test contour, the synthetic
// mutation adapter and the LIVE order version source.
func (w *casesWorld) buildOrderService(adapter actionPort.MutationAdapter, reader actionPort.OwningSystemReader, reconcile actionPort.ReconciliationAdapter, source actionPort.OrderVersionSource, ids *sequenceIDs) *actionApp.Service {
        packs, caps := packresolver.New(), capabilityresolver.New()
        defs := actionCatalog.All()
        if err := actionDomain.CatalogIntegrity(defs); err != nil {
                panic("the product dictionary must pass the guard: " + err.Error())
        }
        for _, def := range defs {
                if err := actionDomain.ValidateDefinitionRefs(def, packs, caps); err != nil {
                        panic("the product dictionary must resolve against the live catalogs: " + err.Error())
                }
        }
        service := actionApp.New(w.identity, packs, caps, defs).
                WithPreviews(actionStore.NewPostgres(runtimePool),
                        controleffectiveresolver.NewRouteAdapter(controlStore.NewPostgres(runtimePool), fixedClock{operationNow}),
                        w.cases, w.identity, w.entitlement, w.employee, orderConnectorFacts{}, w.employee, ids).
                WithAuthorization(actionStore.NewPostgres(runtimePool), actionStore.NewPostgres(runtimePool)).
                WithDispatch(actionStore.NewPostgres(runtimePool), adapter).
                WithReconcile(actionStore.NewPostgres(runtimePool), reconcile).
                WithRecovery(actionStore.NewPostgres(runtimePool)).
                WithOrderVerifications(actionStore.NewPostgres(runtimePool)).
                WithOrderCorrections(actionStore.NewPostgres(runtimePool)).
                // The Story 4.9 status contour: the Supplier Order Fact Receipt
                // and the Order Status Projection read (the fact repository with
                // the fact_no ladder and the UNIQUE full fact key).
                WithOrderSupplierFacts(actionStore.NewPostgres(runtimePool))
        if reader != nil {
                service = service.WithPreviewReader(reader)
        }
        if source != nil {
                service = service.WithOrderVersions(source)
        }
        return service
}

// orderRowCase walks the LIVE 4-2..4-5 chain of ONE row's case: the
// ingested case, the sales mode, the confirmed part request facts, the
// proven fitment, the CURRENT offer of the supplier and the recorded
// client selection — the input state the builder derives the order from.
// Every row of the matrix owns its own case (the AD-7 proposal ladder
// bounds the attempts of one thread — the rows never share a proposal
// key).
func orderRowCase(t *testing.T, w *casesWorld, tag string) (id.UUID, id.UUID) {
        t.Helper()
        ctx := context.Background()
        ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-order-"+tag, "sess-order-"+tag, `{"call":"inbound-call","from":"+79000000797"}`))
        if err != nil {
                t.Fatal(err)
        }
        result := acceptedOutcome(t, ingest)
        caseID := result.CaseID
        detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
        if err != nil {
                t.Fatal(err)
        }
        workItemID := detail.WorkItems[0].WorkItemID
        // The sales mode of the case (the order definition admits the sales
        // conversation only): the turn event with the commercial intent and
        // the classification proposal.
        turn := w.modeTurnEvent(t, caseID, "sess-order-"+tag, "evt-order-"+tag+"-turn", `{"say":"хочу заказать деталь"}`)
        w.proposeMode(t, casesAPI.ProposeModeCommand{
                Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: w.conversationFirst(t, caseID),
                Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
                Reason: "клиент хочет заказать деталь", SignalSource: casesDomain.ModeSignalTurnEvent, TurnRef: turn,
                IdempotencyKey: "order-mode-" + tag, Now: ownershipNow,
        })
        // The confirmed part request material (the exact repeat mode basis).
        for _, material := range []struct {
                key   string
                value []byte
                name  string
        }{
                {"part.kind", []byte(`"фильтр салона"`), "order-fact-kind-" + tag},
                {"part.oem_number", []byte(`"WF2B196K666AA"`), "order-fact-oem-" + tag},
                {"part.quantity", []byte(`2`), "order-fact-quantity-" + tag},
        } {
                fact := partRecordFact(t, w, caseID, workItemID, material.key, material.value, material.name)
                if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
                        Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: fact.FactID,
                        VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
                        Note: "Клиент назвал материал в разговоре", IdempotencyKey: material.name + "-confirm", Now: casesClock.Now(),
                }); err != nil {
                        t.Fatal(err)
                }
        }
        // The proven fitment record of the position (the original link).
        fitmentRecord(t, w, caseID, workItemID, "order-"+tag, nil)
        // The CURRENT offer of the supplier (the full availability).
        offerRecord(t, w, caseID, workItemID, "order-"+tag, func(command *casesAPI.RecordSupplierOfferCommand) {
                command.SupplierID = "SUP-2"
                command.SupplierName = "Поставщик два"
                command.Brand = "Mann"
                command.ClientPrice = 4200
                command.PurchasePrice = nil
                command.QuantityAvailable = 6
                command.IdempotencyKey = "order-offer-" + tag
                command.SourceRecordID = "rec-order-" + tag
        })
        // The recorded client selection (the quantity 2 of the CURRENT offer).
        if _, err := w.cases.RecordClientSelection(ctx, casesAPI.RecordClientSelectionCommand{
                Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
                PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 1, Quantity: 2,
                IdempotencyKey: "order-selection-" + tag, Now: releaseClock.Now(),
        }); err != nil {
                t.Fatalf("the selection command refused: %v", err)
        }
        return caseID, workItemID
}

// orderConfirmAuthorize confirms the built order preview (the step-up
// act) and authorizes the operation (the client key word).
func orderConfirmAuthorize(t *testing.T, w *casesWorld, svc *actionApp.Service, actor identityDomain.Principal,
        caseID, workItemID id.UUID, key string, tag string) (actionAPI.ProposalOutcome, id.UUID, actionDomain.Operation) {
        t.Helper()
        ctx := context.Background()
        built, err := svc.PrepareOrderProposal(ctx, actionAPI.PrepareOrderProposalCommand{
                Actor: actor, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID, Now: orderNow,
        })
        if err != nil {
                t.Fatalf("the order builder refused: %v", err)
        }
        if built.Verdict != actionDomain.VerdictValid || built.VersionNo != 1 {
                t.Fatalf("the built order proposal v1, got %+v", built)
        }
        confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, actor, caseID, built.PreviewID, 1, 3600, orderNow))
        if err != nil {
                t.Fatalf("the order confirmation refused: %v", err)
        }
        authorized, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, actor, caseID, confirmed.ConfirmationID, 1, key, orderNow))
        if err != nil || authorized.Replay || authorized.ReservationReference {
                t.Fatalf("the fresh order authorization, got %+v err=%v", authorized, err)
        }
        view, err := svc.ReadAuthorizationView(ctx, actor, w.tenant, caseID, orderNow)
        if err != nil {
                t.Fatal(err)
        }
        for _, operation := range view.Operations {
                if operation.DefinitionID == actionDomain.OrderActionID && operation.IdempotencyKey == key {
                        return built, confirmed.ConfirmationID, orderOfView(operation)
                }
        }
        t.Fatalf("the authorized order operation of the key %q not found", key)
        return built, confirmed.ConfirmationID, actionDomain.Operation{}
}

// orderOfView rebuilds the domain operation row of the card view (the
// identity pins the rows assert on).
func orderOfView(card actionAPI.OperationCardView) actionDomain.Operation {
        return actionDomain.Operation{
                OperationID: card.OperationID, DefinitionID: card.DefinitionID, DefinitionVersion: card.DefinitionVersion,
                PreviewID: card.PreviewID, PreviewVersionNo: card.PreviewVersionNo,
                ConfirmationID: card.ConfirmationID, ConfirmationVersionNo: card.ConfirmationVersionNo,
                CaseID: card.CaseID, WorkItemID: card.WorkItemID,
        }
}

// orderReceiptOf resolves the LATEST receipt row of one operation over
// the admin truth (the append-only receipts of the outcome ladder).
func orderReceiptOf(t *testing.T, tenant, operationID id.UUID) (outcome string, externalID string) {
        t.Helper()
        if err := adminPool.QueryRow(context.Background(),
                `select outcome, receipt_references->>'external_object_id' from action.action_receipts
                 where tenant_id=$1 and operation_id=$2 order by receipt_version_no desc limit 1`,
                tenant, operationID).Scan(&outcome, &externalID); err != nil {
                return "", ""
        }
        return outcome, externalID
}

// orderLedgerBlockedWord reads the details of the LAST dispatch_blocked
// ledger entry of the operation (the honest parking word).
func orderLedgerBlockedWord(t *testing.T, tenant, operationID id.UUID) string {
        t.Helper()
        var details string
        if err := adminPool.QueryRow(context.Background(),
                `select details from action.operation_ledger
                 where tenant_id=$1 and operation_id=$2 and kind='dispatch_blocked'
                 order by entry_no desc limit 1`, tenant, operationID).Scan(&details); err != nil {
                return ""
        }
        return details
}

// fmtOrderTarget renders the deterministic idempotency key of one matrix
// row (the distinct client key words derive distinct business intents).
func fmtOrderTarget(row int64) string { return fmt.Sprintf("order-client-key-%d", row) }

// TestOrderMatrixRowsBySpec walks the Story 4.6 eval matrix LIVE: the
// full chain row (AC-15 core) plus the by-name rows — the double
// confirmation (the superseded fence BY NAME), the command replay (the
// idempotent authorization), the saved receipt of the duplicate request,
// the timeout after the dispatch (the honest unknown — the blind repeat
// forbidden), the changed price (the live OrderDigest divergence blocks
// the dispatch with the exact word), the vanished availability (the same
// divergence through the re-recorded offer), the revoked confirmation
// (the old-consent fence BY NAME), the recovery case of the partial
// result (the exhausted reconciliation of 3-7 → the recovery case of
// 3-8), the unproven manager fence (the nil OrderVersionSource fails
// CLOSED with the exact word) and the RLS blindness of the foreign
// tenant.
func TestOrderMatrixRowsBySpec(t *testing.T) {
        if runtimePool == nil {
                t.Skip("no runtime pool")
        }
        ctx := context.Background()
        w, svc, adapter, reader, reconcile, operator := orderWorld(t, 757)
        _ = reader
        // Every row owns its own case: the per-row chain of the 4-2..4-5
        // inputs (the AD-7 proposal ladder bounds one thread — the rows never
        // share a proposal key).
        caseID, workItemID := orderRowCase(t, w, "1")

        // Row 1 — the FULL live chain: the builder proposal (the typed input
        // carries the canonical OrderDigest over the pins of ALL FIVE inputs,
        // the material render with the MANDATORY manager-check word, the known
        // sum 2 × 4200 = 8400), the step-up confirmation, the machine
        // authorization, the synthetic dispatch (committed with the evidence
        // triple) and the receipt with the derived words of the order state.
        t.Run("row-1 full chain: builder→confirm→authorize→dispatch→receipt", func(t *testing.T) {
                built, _, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(1), "1")
                // The typed input of the built preview: the digest, the material
                // render with the manager-check word, the sum and the currency.
                preview, err := svc.Get(ctx, operator, w.tenant, actionDomain.OrderActionID, actionDomain.OrderActionVersion)
                _ = preview
                if err != nil {
                        t.Fatal(err)
                }
                thread, err := svc.ReadPreviewView(ctx, operator, w.tenant, caseID)
                if err != nil {
                        t.Fatal(err)
                }
                var digest, material, sum, currency string
                found := false
                for _, threadView := range thread.Threads {
                        for _, card := range threadView.Cards {
                                if card.PreviewID != built.PreviewID || card.VersionNo != 1 {
                                        continue
                                }
                                found = true
                                digest = card.TypedInput["order_digest"]
                                material = card.TypedInput["order_material"]
                                sum = card.TypedInput["known_client_sum"]
                                currency = card.TypedInput["currency"]
                        }
                }
                if !found || len(digest) != 64 || sum != "8400.00" || currency != "RUB" {
                        t.Fatalf("the built order typed input: digest=%q sum=%q currency=%q", digest, sum, currency)
                }
                if !strings.Contains(material, "Mann / WF2B196K666AA") || !strings.Contains(material, "8400.00 RUB") {
                        t.Fatalf("the material render carries the position and the sum, got:\n%s", material)
                }
                if !strings.Contains(material, actionDomain.OrderManagerCheckWord) {
                        t.Fatalf("the MANDATORY manager-check word rides the material (AC-1), got:\n%s", material)
                }
                // The synthetic dispatch: committed with the complete evidence
                // triple; the receipt carries the external id, the derived state
                // word, the client words, the 1C-unknown and the assignment word.
                adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "corr-order-1", nil)
                adapter.inner.ScriptEvidence("ABCP-ORDER-42", "3")
                adapter.reset()
                report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(10 * time.Minute),
                })
                if err != nil {
                        t.Fatal(err)
                }
                if report.Dispatched != 1 {
                        t.Fatalf("the committed order dispatch, got %+v", report)
                }
                if adapter.total() != 1 {
                        t.Fatalf("ONE adapter call, got %d", adapter.total())
                }
                view, err := svc.ReadAuthorizationView(ctx, operator, w.tenant, caseID, orderNow)
                if err != nil {
                        t.Fatal(err)
                }
                var receipt *actionAPI.ActionReceiptView
                for _, card := range view.Operations {
                        if card.OperationID != operation.OperationID {
                                continue
                        }
                        for index := range card.Receipts {
                                if card.Receipts[index].Outcome == actionDomain.ReceiptOutcomeCommitted {
                                        receipt = &card.Receipts[index]
                                }
                        }
                }
                if receipt == nil {
                        t.Fatalf("the committed order receipt expected, operations %+v", view.Operations)
                }
                if receipt.ExternalObjectID != "ABCP-ORDER-42" || receipt.ExternalObjectVersion != "3" {
                        t.Fatalf("the external order triple, got %+v", receipt)
                }
                if receipt.OrderState != actionDomain.OrderReceiptStatePendingManagerVerification {
                        t.Fatalf("the derived order state, got %q", receipt.OrderState)
                }
                if receipt.OrderStateWord != actionDomain.OrderClientCreatedWord || receipt.OrderBoundaryWord != actionDomain.OrderClientBoundaryWord {
                        t.Fatalf("the client words, got %+v", receipt)
                }
                if receipt.OrderOneCWord != actionDomain.OrderOneCUnknownWord || receipt.OrderManagerWord != actionDomain.OrderManagerAssignmentWord {
                        t.Fatalf("the 1C word and the assignment word, got %+v", receipt)
                }
        })

        // Row 2 — the DOUBLE confirmation: the second confirmation of the same
        // thread supersedes the first — the operation pinned to v1 hits the
        // superseded fence BY NAME (the old consent never authorizes the
        // dispatch, AC-3).
        t.Run("row-2 double confirmation: superseded BY NAME", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "2")
                built, _, _ := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(2), "2")
                adapter.reset()
                // The client re-selects the position (a NEW selection snapshot) and
                // the builder re-derives: the attempt 2 of the same proposal thread
                // builds preview VERSION 2 (the AD-7 repair ladder — the new exact
                // version of the composition).
                if _, err := w.cases.RecordClientSelection(ctx, casesAPI.RecordClientSelectionCommand{
                        Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
                        PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 1, Quantity: 3,
                        IdempotencyKey: "order-selection-2b", Now: releaseClock.Now().Add(time.Minute),
                }); err != nil {
                        t.Fatalf("the re-selection must land: %v", err)
                }
                rebuilt, err := svc.PrepareOrderProposal(ctx, actionAPI.PrepareOrderProposalCommand{
                        Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID, Now: orderNow.Add(2 * time.Minute),
                })
                if err != nil || rebuilt.Verdict != actionDomain.VerdictValid || rebuilt.VersionNo != 2 {
                        t.Fatalf("the re-derived order preview v2, got %+v err=%v", rebuilt, err)
                }
                // The second confirmation of the NEW preview version 2: version 2
                // of the confirmation thread supersedes version 1 (the old consent
                // never authorizes the dispatch, AC-3).
                if _, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, operator, caseID, built.PreviewID, 2, 3600, orderNow.Add(2*time.Minute))); err != nil {
                        t.Fatalf("the second confirmation must land: %v", err)
                }
                report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(12 * time.Minute),
                })
                if err != nil {
                        t.Fatal(err)
                }
                if report.Blocked == 0 {
                        t.Fatalf("the superseded consent blocks the dispatch, got %+v", report)
                }
                if adapter.total() != 0 {
                        t.Fatalf("the superseded consent NEVER sends, got %d calls", adapter.total())
                }
        })

        // Row 3 — the COMMAND REPLAY: the same client key returns the SAME
        // operation (the replay flag, no second row) — and after the committed
        // outcome the duplicate request keeps the SAVED receipt (the new ABCP
        // Order is never created, AC-6/AC-11).
        t.Run("row-3 command replay + duplicate keeps the saved receipt", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "3")
                _, confirmationID, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(3), "3")
                adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "corr-order-3", nil)
                adapter.inner.ScriptEvidence("ABCP-ORDER-43", "3")
                adapter.reset()
                if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(14 * time.Minute),
                }); err != nil {
                        t.Fatal(err)
                }
                if adapter.total() != 1 {
                        t.Fatalf("the committed dispatch of the row, got %d calls", adapter.total())
                }
                // The duplicate authorization: the same key replays the SAME
                // operation — no second row, no second dispatch.
                replayed, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, operator, caseID, confirmationID, 1, fmtOrderTarget(3), orderNow.Add(15*time.Minute)))
                if err != nil || !replayed.Replay {
                        t.Fatalf("the duplicate request replays the saved operation, got %+v err=%v", replayed, err)
                }
                adapter.reset()
                if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(16 * time.Minute),
                }); err != nil {
                        t.Fatal(err)
                }
                if adapter.total() != 0 {
                        t.Fatalf("the duplicate request NEVER re-sends, got %d calls", adapter.total())
                }
                outcome, externalID := orderReceiptOf(t, w.tenant, operation.OperationID)
                if outcome != actionDomain.ReceiptOutcomeCommitted || externalID != "ABCP-ORDER-43" {
                        t.Fatalf("the SAVED receipt of the duplicate, got %q %q", outcome, externalID)
                }
        })

        // Row 4 — the TIMEOUT after the dispatch: the broken adapter leaves
        // the honest unknown; WITHOUT the live guarantee the retry policy
        // parks the operation (the blind repeat is forbidden, AC-10) — the
        // receipt stays unknown, the client hears neither success nor refusal.
        t.Run("row-4 timeout: unknown, no blind repeat", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "4")
                _, _, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(4), "4")
                adapter.inner.Script("", "", errors.New("адаптер недоступен: обрыв соединения до провайдера"))
                adapter.reset()
                if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(18 * time.Minute),
                }); err != nil {
                        t.Fatal(err)
                }
                if adapter.total() != 1 {
                        t.Fatalf("the broken send happened once, got %d", adapter.total())
                }
                outcome, _ := orderReceiptOf(t, w.tenant, operation.OperationID)
                if outcome != actionDomain.ReceiptOutcomeUnknown && outcome != "" {
                        t.Fatalf("the honest unknown of the timeout, got %q", outcome)
                }
                // The re-drain: the recorded guarantee word decides; the synthetic
                // adapter recorded NO live guarantee — the retry parks, the second
                // blind call never happens.
                adapter.reset()
                report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(19 * time.Minute),
                })
                if err != nil {
                        t.Fatal(err)
                }
                if adapter.total() != 0 {
                        t.Fatalf("the blind repeat is forbidden, got %d calls", adapter.total())
                }
                _ = report
        })

        // Row 5 — the CHANGED PRICE: after the confirmation the supplier
        // re-records the offer (a NEW offer number with the new price) — the
        // LIVE OrderDigest diverges from the pinned preview material and the
        // dispatch parks with the exact composition word (AC-5; never a false
        // failed externally, FR-28).
        t.Run("row-5 changed price: the live digest blocks the dispatch", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "5")
                _, _, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(5), "5")
                // The supplier re-records: the new price 4300 under a NEW offer
                // number — the CURRENT record of the pair moves on.
                offerRecord(t, w, caseID, workItemID, "order-5-new", func(command *casesAPI.RecordSupplierOfferCommand) {
                        command.SupplierID = "SUP-2"
                        command.SupplierName = "Поставщик два"
                        command.Brand = "Mann"
                        command.ClientPrice = 4300
                        command.PurchasePrice = nil
                        command.QuantityAvailable = 6
                        command.IdempotencyKey = "order-offer-5-new"
                        command.SourceRecordID = "rec-order-5-new"
                })
                adapter.reset()
                report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(20 * time.Minute),
                })
                if err != nil {
                        t.Fatal(err)
                }
                if report.Blocked == 0 {
                        t.Fatalf("the changed price blocks the dispatch, got %+v", report)
                }
                if adapter.total() != 0 {
                        t.Fatalf("the changed composition NEVER sends, got %d calls", adapter.total())
                }
                word := orderLedgerBlockedWord(t, w.tenant, operation.OperationID)
                if !strings.Contains(word, actionDomain.OrderCompositionChangedWord) {
                        t.Fatalf("the exact composition word of the block, got %q", word)
                }
        })

        // Row 6 — the VANISHED AVAILABILITY: the supplier re-records the offer
        // with the dropped availability — the CURRENT record changes (the
        // OfferDigest carries the availability) and the same honest divergence
        // blocks the dispatch (AC-5; the 4-5 guard refuses the NEW selection
        // over the shortfall — the split never happens silently).
        t.Run("row-6 vanished availability: the same divergence word", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "6")
                _, _, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(6), "6")
                offerRecord(t, w, caseID, workItemID, "order-6-new", func(command *casesAPI.RecordSupplierOfferCommand) {
                        command.SupplierID = "SUP-2"
                        command.SupplierName = "Поставщик два"
                        command.Brand = "Mann"
                        command.ClientPrice = 4200
                        command.PurchasePrice = nil
                        command.QuantityAvailable = 1
                        command.IdempotencyKey = "order-offer-6-new"
                        command.SourceRecordID = "rec-order-6-new"
                })
                adapter.reset()
                if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(22 * time.Minute),
                }); err != nil {
                        t.Fatal(err)
                }
                if adapter.total() != 0 {
                        t.Fatalf("the vanished availability NEVER sends, got %d calls", adapter.total())
                }
                word := orderLedgerBlockedWord(t, w.tenant, operation.OperationID)
                if !strings.Contains(word, actionDomain.OrderCompositionChangedWord) {
                        t.Fatalf("the exact composition word of the availability change, got %q", word)
                }
        })

        // Row 7 — the REVOKED confirmation: the human revocation of the exact
        // consent version kills the FUTURE dispatch — the revoked fence BY
        // NAME (the old consent never sends, AC-3).
        t.Run("row-7 revoked confirmation: revoked BY NAME", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "7")
                _, confirmationID, _ := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(7), "7")
                if _, err := svc.RevokeConfirmation(ctx, operationRevokeCommand(w, operator, caseID, confirmationID, 1, orderNow.Add(2*time.Minute))); err != nil {
                        t.Fatalf("the revocation must land: %v", err)
                }
                adapter.reset()
                report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(24 * time.Minute),
                })
                if err != nil {
                        t.Fatal(err)
                }
                if report.Blocked == 0 {
                        t.Fatalf("the revoked consent blocks the dispatch, got %+v", report)
                }
                if adapter.total() != 0 {
                        t.Fatalf("the revoked consent NEVER sends, got %d calls", adapter.total())
                }
        })

        // Row 8 — the PARTIAL result: the broken dispatch leaves the honest
        // unknown and the THREE no-answer reconciliation attempts exhaust —
        // the recovery case of 3-8 is created with the order operation pin
        // (AC-12; the manager receives the material, the order never moves to
        // the supplier).
        t.Run("row-8 partial result: the recovery case of the exhausted unknown", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "8")
                _, _, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, fmtOrderTarget(8), "8")
                adapter.inner.Script("", "", errors.New("адаптер недоступен: обрыв соединения до провайдера"))
                adapter.reset()
                if _, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(26 * time.Minute),
                }); err != nil {
                        t.Fatal(err)
                }
                // The THREE no-answer reconciliation attempts of the bounded
                // exhaustion (the 3-7 ladder) — the recovery case lands.
                reconcile.Script("", "", errors.New("reconciliation adapter unreachable"))
                for minute := 60; minute <= 140; minute += 40 {
                        if _, err := svc.ReconcileUnknownOperations(ctx, actionApp.ReconcileCommand{
                                Actor: operator, TenantID: w.tenant, Now: orderNow.Add(time.Duration(minute) * time.Minute),
                        }); err != nil {
                                t.Fatalf("the no_answer attempt of the minute %d must answer: %v", minute, err)
                        }
                }
                // The creation command of the exhausted unknown (the 3-8 worker —
                // the trigger reads the spent attempts): the recovery case carries
                // the order operation pin and the honest exhausted trigger.
                created, err := svc.CreateRecoveryCase(ctx, actionAPI.RecoveryCreateCommand{
                        Actor: operator, TenantID: w.tenant, OperationID: operation.OperationID,
                        Now: orderNow.Add(200 * time.Minute),
                })
                if err != nil {
                        t.Fatalf("the exhausted unknown must create the recovery case: %v", err)
                }
                if created.Decision != actionPort.RecoveryDecisionCreated {
                        t.Fatalf("the creation decision, got %q", created.Decision)
                }
                if created.Case.Trigger != actionDomain.RecoveryTriggerReconciliationExhausted {
                        t.Fatalf("the exhausted trigger of the order recovery, got %q", created.Case.Trigger)
                }
                var recoveries int
                if err := adminPool.QueryRow(ctx, `select count(*) from action.recovery_cases where tenant_id=$1 and operation_id=$2`,
                        w.tenant, operation.OperationID).Scan(&recoveries); err != nil {
                        t.Fatal(err)
                }
                if recoveries == 0 {
                        t.Fatal("the exhausted unknown must own its recovery case (3-8)")
                }
        })

        // Row 9 — the UNPROVEN manager fence: the nil OrderVersionSource fails
        // CLOSED — the dispatch of the confirmed order blocks with the exact
        // word of the missing live source (the AG-4 boundary; the recommendation
        // of the order_creation manager task follows — row 9b).
        t.Run("row-9 nil source: fail-closed with the exact word", func(t *testing.T) {
                caseID, workItemID := orderRowCase(t, w, "9")
                // The AG-4 composition from the start of the row's own chain: the
                // SAME stores with NO order version source — the honest production
                // boundary of the port. The whole chain (the builder, the consent,
                // the authorization) runs GREEN; the DISPATCH fails CLOSED.
                ag4 := w.buildOrderService(adapter, reader, reconcile, nil, actionPreviewIDs(7589, 512))
                built, err := ag4.PrepareOrderProposal(ctx, actionAPI.PrepareOrderProposalCommand{
                        Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID, Now: orderNow,
                })
                if err != nil || built.Verdict != actionDomain.VerdictValid || built.VersionNo != 1 {
                        t.Fatalf("the ag4 builder builds the proposal green, got %+v err=%v", built, err)
                }
                confirmed, err := ag4.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, operator, caseID, built.PreviewID, 1, 3600, orderNow))
                if err != nil {
                        t.Fatal(err)
                }
                authorized, err := ag4.AuthorizeOperation(ctx, operationAuthorizeCommand(w, operator, caseID, confirmed.ConfirmationID, 1, fmtOrderTarget(9), orderNow))
                if err != nil || authorized.Replay || authorized.ReservationReference {
                        t.Fatalf("the fresh authorization over the AG-4 service, got %+v err=%v", authorized, err)
                }
                adapter.reset()
                report, err := ag4.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
                        Actor: operator, TenantID: w.tenant, Now: orderNow.Add(28 * time.Minute),
                })
                if err != nil {
                        t.Fatal(err)
                }
                if report.Blocked == 0 {
                        t.Fatalf("the nil source blocks the dispatch, got %+v", report)
                }
                if adapter.total() != 0 {
                        t.Fatalf("the fail-closed fence NEVER sends, got %d calls", adapter.total())
                }
                view, err := ag4.ReadAuthorizationView(ctx, operator, w.tenant, caseID, orderNow)
                if err != nil {
                        t.Fatal(err)
                }
                found := false
                for _, card := range view.Operations {
                        if card.IdempotencyKey != fmtOrderTarget(9) {
                                continue
                        }
                        found = true
                        word := orderLedgerBlockedWord(t, w.tenant, card.OperationID)
                        if !strings.Contains(word, actionDomain.OrderVersionSourceMissingWord) {
                                t.Fatalf("the fail-closed word of the missing source, got %q", word)
                        }
                }
                if !found {
                        t.Fatal("the parked operation of the nil-source row")
                }
                // Row 9b — the RECOMMENDATION: the order_creation handoff reason
                // word lands through the EXISTING request_human machine (the SQL
                // mirror 00025 admits it; the panel recommends, the operator
                // decides — the decision-ready task).
                turnRef := w.modeTurnEvent(t, caseID, "sess-order-9", "evt-order-9-turn-h", `{"say":"заказ создан, нужна проверка менеджера"}`)
                liveMode := w.modeView(t, caseID)
                handoff, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
                        Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID,
                        TurnRef:             turnRef,
                        ExpectedModeVersion: liveMode.Active.ModeVersion,
                        TargetRoles:         []string{"operator"},
                        AckDeadline:         "ack_4h",
                        Reason:              casesDomain.HandoffReasonOrderCreation,
                        IdempotencyKey:      "order-handoff-9", Now: casesClock.Now(),
                })
                if err != nil {
                        t.Fatalf("the order_creation handoff must land (00025): %v", err)
                }
                if handoff.Command != "request_human" || handoff.ModeTermID == "" || handoff.HandoffTurnRef == "" {
                        t.Fatalf("the handoff outcome, got %+v", handoff)
                }
        })

        // Row 10 — the RLS blindness: the foreign tenant never reads the
        // order material (the 404 of the foreign scope; no selection rows of
        // the foreign view).
        t.Run("row-10 RLS blindness of the foreign tenant", func(t *testing.T) {
                foreign := newCasesWorld(t, 758)
                foreignCase, _ := foreign.acceptedCase(t, "evt-order-foreign", "sess-order-foreign", `{"call":"inbound-call","from":"+79000000798"}`)
                // The foreign world reads ITS OWN case — no order material of the
                // source tenant ever appears.
                detail, err := foreign.cases.CaseView(ctx, foreign.workActor(), foreign.tenant, foreignCase)
                if err != nil {
                        t.Fatal(err)
                }
                if len(detail.Comparison.Selections) != 0 {
                        t.Fatalf("the foreign case carries no selections of the source tenant, got %d", len(detail.Comparison.Selections))
                }
                // The foreign principal reads the SOURCE case through the action
                // surface: the RLS-blind empty view — zero operations, zero
                // confirmations, zero receipts of the source tenant (the honest
                // blindness, never an error and never a leak).
                foreignView, err := svc.ReadAuthorizationView(ctx, foreign.workActor(), foreign.tenant, caseID, orderNow)
                if err != nil {
                        t.Fatalf("the foreign read of the foreign tenant is the empty view, got %v", err)
                }
                if len(foreignView.Operations) != 0 || len(foreignView.Confirmations) != 0 {
                        t.Fatalf("the foreign view carries no source-tenant rows, got %d operations", len(foreignView.Operations))
                }
                var selections int
                if err := adminPool.QueryRow(ctx, `select count(*) from cases.selection_snapshots where tenant_id=$1 and case_id=$2`,
                        foreign.tenant, foreignCase).Scan(&selections); err != nil {
                        t.Fatal(err)
                }
                if selections != 0 {
                        t.Fatalf("the foreign tenant owns no selection rows, got %d", selections)
                }
        })
}
