package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	"github.com/staffy-co/staffy/internal/adapters/actiondispatcher"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	entitlementAPI "github.com/staffy-co/staffy/internal/entitlement/api"
	entitlementDomain "github.com/staffy-co/staffy/internal/entitlement/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// Story 4.11 (step-04): the pilot outcome contour of the journeys world
// — the delegation of the narrow READ-ONLY surface the transport
// type-asserts (the servicing precedent) plus the world seeding of the
// z48 journey. The seeding walks the REAL commands of the fixture world
// through the SAME honest machine the z43..z47 journeys walk from the
// browser: one accepted inbound channel event (the attribution material
// of the outcome — only what the source carried), the confirmed vehicle
// profile of 4-2, the confirmed part request of 4-3, the sales mode of
// the conversation, the proven fitment evidence of 4-4, the LIVE offer
// and the client selection of 4-5, the committed order receipt with the
// external ABCP order ID of 4-6, the approved verification decision of
// 4-7 and the supplier {formed, sent} facts of 4-9 (the
// sent-after-formed ladder of the same supplier order ID) — the
// FULL-JOURNEY case; plus ONE second FRESH case (the inquiry-only row of
// the honest funnel — no material beyond the accepted inbound event).
// The /fixture/pilot-outcome-world toggle seeds ONCE (the
// /fixture/controls precedent — the toggle seeds through the real Set
// paths; the append-only history never unseeds) and resets the order
// world routing on the off leg (the /fixture/order-world family reset).
//
// The seed register of the order family (the UNIQUE provider event ids
// and the idempotency keys every journey owns): z43 took
// evt-o1/sess-o1 + journey-order-key-*, z44 evt-v44/sess-v44 +
// journey-verify-key-*, z45 evt-v45/sess-v45 +
// journey-correct-key-*, z46 evt-v46/sess-v46 +
// journey-status-key-* (ABCP-ORDER-46/SUP-46), z47 evt-v47/sess-v47 +
// journey-servicing-key-* (ABCP-ORDER-47/SUP-47) — THIS journey takes
// the NEXT FREE name of the family: evt-v48/sess-v48 (plus the -fresh
// suffix pair of the second case) + journey-pilot-key-* +
// ABCP-ORDER-48/SUP-48/journey-pilot-corr-48.

// pilotOutcomeNotWiredWord is the fixture mirror of the honest refusal
// word of the unwired contour (the app gate speaks the same word — the
// transport word-match degrades BOTH surfaces the designed honest way:
// the zero panel and the zero page, never a nil panic).
const pilotOutcomeNotWiredWord = "контур исхода обращения не подключён: сверьте конфигурацию поверхности"

// The seed names of the z48 journey (the order-family register above).
const (
	pilotSeedEventID  = "journey-offers-evt-v48"
	pilotSeedSession  = "journey-offers-sess-v48"
	pilotFreshEventID = "journey-offers-evt-v48-fresh"
	pilotFreshSession = "journey-offers-sess-v48-fresh"
	pilotSeedKey      = "journey-pilot-key-42"
	pilotExternalID   = "ABCP-ORDER-48"
	pilotSupplierID   = "SUP-48"
	pilotCorrelation  = "journey-pilot-corr-48"
)

// pilotOutcomeFixture is the world builder of the z48 journey: the
// handles of the fixture world (the cases service, the actions routing,
// the order world service and its dispatch adapter, the entitlement
// admission) plus the once-seeded anchor ids of the two cases the
// journey reads.
type pilotOutcomeFixture struct {
	mu          sync.Mutex
	seeded      bool
	caseID      id.UUID
	freshCaseID id.UUID
	cases       *casesFixture
	actions     *actionsFixture
	order       *actionApp.Service
	dispatch    *actiondispatcher.Synthetic
	entitlement *entitlementFixture
}

// newPilotOutcomeFixture wires the world builder over the LIVE fixture
// instances the server assembly owns.
func newPilotOutcomeFixture(cases *casesFixture, actions *actionsFixture, order *actionApp.Service, dispatch *actiondispatcher.Synthetic, entitlement *entitlementFixture) *pilotOutcomeFixture {
	return &pilotOutcomeFixture{cases: cases, actions: actions, order: order, dispatch: dispatch, entitlement: entitlement}
}

// pilotService picks the order world: the Story 4.6 composition when its
// toggle chose it (the same routing as every order-family surface);
// every other contour answers the honest not-wired word of the pilot
// contour (the mirror of the app gate — both read surfaces degrade the
// designed zero way).
func (f *actionsFixture) pilotService() *actionApp.Service {
	if f.orderEnabled && f.order != nil {
		return f.order
	}
	return nil
}

// ReadPilotCaseOutcome delegates to the order world's pilot contour (the
// status/servicing delegation precedent — the READ-ONLY derivation; the
// journeys walk the SAME honest machine).
func (f *actionsFixture) ReadPilotCaseOutcome(ctx context.Context, actor identityDomain.Principal, tenantID, caseID id.UUID, now time.Time) (actionDomain.PilotCaseOutcome, error) {
	f.mu.Lock()
	service := f.pilotService()
	f.mu.Unlock()
	if service == nil {
		return actionDomain.PilotCaseOutcome{}, fmt.Errorf("%s", pilotOutcomeNotWiredWord)
	}
	return service.ReadPilotCaseOutcome(ctx, actor, tenantID, caseID, now)
}

// ReadPilotReport delegates to the order world's pilot contour (the
// aggregate period report — READ-ONLY).
func (f *actionsFixture) ReadPilotReport(ctx context.Context, actor identityDomain.Principal, tenantID id.UUID, period string, now time.Time) (actionDomain.PilotReport, error) {
	f.mu.Lock()
	service := f.pilotService()
	f.mu.Unlock()
	if service == nil {
		return actionDomain.PilotReport{}, fmt.Errorf("%s", pilotOutcomeNotWiredWord)
	}
	return service.ReadPilotReport(ctx, actor, tenantID, period, now)
}

// seed builds the world of the z48 journey ONCE through the REAL
// commands (the idempotent replay of the toggle answers the SAME two
// anchors — the append-only history never rewrites). The chain mirrors
// the z43..z47 walks: the entitlement admission of the order machine,
// the accepted inbound event of the offers world (the 6.0.0 pack pin —
// the vehicle/part/fitment/comparison panels all derive over it), the
// confirmed identification facts, the sales mode, the proven fitment,
// the LIVE offer and the client selection, the order
// build→confirm→authorize→committed-dispatch ladder with the external
// ABCP order ID, the verification assign→acknowledge→approve decision
// over the live digest and the supplier formed/sent facts; then the
// second FRESH case of the inquiry-only funnel row.
func (p *pilotOutcomeFixture) seed(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seeded {
		return nil
	}
	actor := principalOfTheWorld()
	// The pilot surfaces ride the order world composition while the flag
	// is on (the /fixture/order-world toggle — the reads of the journey
	// route through the same delegation).
	p.actions.setOrder(true)
	// The LIVE entitlement admission of the order machine (the z47 §0
	// walk, server-side): the builder admits the active contour only —
	// draft/paused answer blocked_new_admissions.
	if err := p.ensureEntitlement(ctx, actor); err != nil {
		return err
	}
	now := journeyClock.Now()

	// 1. The FULL-JOURNEY case: the accepted inbound channel event of the
	// offers world — the attribution material of the outcome (the FIRST
	// accepted inbound event wins the attribution inside the derivation).
	full, err := p.ingestPilotCase(ctx, pilotSeedEventID, pilotSeedSession,
		`{"say":"Хочу заказать салонный фильтр WF2B196K666AA, нужно 2 штуки"}`, now)
	if err != nil {
		return err
	}
	detail, err := p.cases.CaseView(ctx, actor, tenantID, full.CaseID)
	if err != nil {
		return err
	}
	if len(detail.WorkItems) == 0 {
		return fmt.Errorf("the pilot case carries no work item")
	}
	workItemID := detail.WorkItems[0].WorkItemID

	// 2. The vehicle identification facts of 4-2 (the four keys of the
	// schema) with the operator confirmations of the exact versions — the
	// confirmed profile satisfies the alternative sets of the schema.
	for _, material := range []struct {
		key   string
		value string
		name  string
	}{
		{"vehicle.vin", `"WF0XXXTT4S1234567"`, "pilot-vehicle-vin-48"},
		{"vehicle.make", `"Ford"`, "pilot-vehicle-make-48"},
		{"vehicle.model", `"Focus"`, "pilot-vehicle-model-48"},
		{"vehicle.year_of_manufacture", `"2021"`, "pilot-vehicle-year-48"},
	} {
		fact, err := p.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
			Actor: actor, TenantID: tenantID, CaseID: full.CaseID, WorkItemID: workItemID,
			MaterialKey: material.key, Value: []byte(material.value),
			SourceType: casesDomain.FactSourceOperator, SourceID: material.name,
			Reason: "Клиент продиктовал данные автомобиля", IdempotencyKey: material.name, Now: now,
		})
		if err != nil {
			return fmt.Errorf("the vehicle fact %s: %w", material.key, err)
		}
		if _, err := p.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
			Actor: actor, TenantID: tenantID, CaseID: full.CaseID, FactID: fact.FactID,
			VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
			Note: "Данные автомобиля подтверждены в разговоре", IdempotencyKey: material.name + "-confirm", Now: now,
		}); err != nil {
			return fmt.Errorf("the vehicle confirmation %s: %w", material.key, err)
		}
	}

	// 3. The part request facts of 4-3 with the confirmations (the request
	// pin of the fitment evidence and the order material).
	for _, material := range []struct {
		key   string
		value string
		name  string
	}{
		{"part.kind", `"фильтр салона"`, "pilot-part-kind-48"},
		{"part.oem_number", `"WF2B196K666AA"`, "pilot-part-oem-48"},
		{"part.quantity", `2`, "pilot-part-quantity-48"},
	} {
		fact, err := p.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
			Actor: actor, TenantID: tenantID, CaseID: full.CaseID, WorkItemID: workItemID,
			MaterialKey: material.key, Value: []byte(material.value),
			SourceType: casesDomain.FactSourceOperator, SourceID: material.name,
			Reason: "Клиент назвал деталь в разговоре", IdempotencyKey: material.name, Now: now,
		})
		if err != nil {
			return fmt.Errorf("the part fact %s: %w", material.key, err)
		}
		if _, err := p.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
			Actor: actor, TenantID: tenantID, CaseID: full.CaseID, FactID: fact.FactID,
			VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
			Note: "Материал запроса детали подтверждён", IdempotencyKey: material.name + "-confirm", Now: now,
		}); err != nil {
			return fmt.Errorf("the part confirmation %s: %w", material.key, err)
		}
	}

	// 4. The sales mode of the conversation (the order definition admits
	// the sales contour only): the turn signal over the accepted event —
	// the unclassified→sales raise of the mode ladder.
	if _, err := p.cases.ProposeMode(ctx, casesAPI.ProposeModeCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, ConversationID: full.ConversationID,
		Mode: casesDomain.ModeSales, Scope: casesDomain.ModeScopeConversation,
		ExpectedModeVersion: 1, Reason: "клиент хочет заказать деталь",
		SignalSource: casesDomain.ModeSignalTurnEvent, TurnRef: full.EventID,
		IdempotencyKey: "pilot-mode-48", Now: now,
	}); err != nil {
		return fmt.Errorf("the sales mode proposal: %w", err)
	}

	// 5. The proven fitment evidence of 4-4 (the original link of the
	// catalog research).
	if _, err := p.cases.RecordFitmentEvidence(ctx, casesAPI.RecordFitmentEvidenceCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, WorkItemID: workItemID,
		PositionKey: "WF2B196K666AA", CatalogNode: "Фильтр салона", Article: "WF2B196K666AA",
		LinkType: casesDomain.FitmentLinkOriginal, Chain: []string{"Фильтр салона (оригинальная позиция)"},
		CatalogVehicleIDs: []string{"ABCP-12345"},
		SourceSystem:      casesDomain.FitmentSourceAbcpCatalog, SourceRecordID: "rec-pilot-fitment-48",
		SourceVersion: "v2026.10", SourceObservedAt: now.Add(-2 * time.Hour),
		IdempotencyKey: "pilot-fitment-48", Now: now,
	}); err != nil {
		return fmt.Errorf("the fitment evidence: %w", err)
	}

	// 6. The LIVE supplier offer of 4-5 (the purchase price stays OUTSIDE
	// the contour — the 4-9 fence: the honest nil) and the client
	// selection of the current offer.
	if _, err := p.cases.RecordSupplierOffer(ctx, casesAPI.RecordSupplierOfferCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, WorkItemID: workItemID,
		PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", SupplierName: "Поставщик два",
		Brand: "Mann", Article: "WF2B196K666AA", ClientPrice: 4200, PurchasePrice: nil,
		Currency: "RUB", QuantityAvailable: 6, TermDays: intPtr(1), Conditions: "самовывоз со склада",
		SourceSystem: casesDomain.FitmentSourceAbcpCatalog, SourceRecordID: "rec-pilot-48",
		SourceVersion: "v2026.10", SourceObservedAt: now.Add(-90 * time.Minute),
		IdempotencyKey: "pilot-offer-48", Now: now,
	}); err != nil {
		return fmt.Errorf("the supplier offer: %w", err)
	}
	if _, err := p.cases.RecordClientSelection(ctx, casesAPI.RecordClientSelectionCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, WorkItemID: workItemID,
		PositionKey: "WF2B196K666AA", SupplierID: "SUP-2", OfferNo: 1, Quantity: 2,
		IdempotencyKey: "pilot-selection-48", Now: now,
	}); err != nil {
		return fmt.Errorf("the client selection: %w", err)
	}

	// 7. The order machine of 4-6: the LIVE builder over the recorded
	// material, the step-up confirmation, the authorization with the
	// journey key and the COMMITTED dispatch carrying the external ABCP
	// order ID (the scripted evidence triple of the synthetic adapter —
	// the honest test-contour class of the fixture world).
	built, err := p.order.PrepareOrderProposal(ctx, actionAPI.PrepareOrderProposalCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, WorkItemID: workItemID, Now: now,
	})
	if err != nil {
		return fmt.Errorf("the order builder: %w", err)
	}
	if built.Verdict != actionDomain.VerdictValid {
		return fmt.Errorf("the order builder verdict: %s %+v", built.Verdict, built.Violations)
	}
	confirmed, err := p.order.ConfirmPreviewVersion(ctx, actionAPI.ConfirmPreviewVersionCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID,
		PreviewID: built.PreviewID, PreviewVersionNo: built.VersionNo,
		LifetimeSeconds: 600, Now: now,
	})
	if err != nil {
		return fmt.Errorf("the order confirmation: %w", err)
	}
	if _, err := p.order.AuthorizeOperation(ctx, actionAPI.AuthorizeOperationCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID,
		ConfirmationID: confirmed.ConfirmationID, ConfirmationVersionNo: confirmed.VersionNo,
		IdempotencyKey: pilotSeedKey, Now: now,
	}); err != nil {
		return fmt.Errorf("the order authorization: %w", err)
	}
	p.dispatch.ScriptEvidence(pilotExternalID, "3")
	p.dispatch.ScriptGuarantee(true)
	p.dispatch.Script(actionDomain.MutationOutcomeCommitted, pilotCorrelation, nil)
	drained, err := p.order.DrainDispatchQueue(ctx, actionAPI.DrainDispatchCommand{
		Actor: actor, TenantID: tenantID, Now: now.Add(10 * time.Minute),
	})
	if err != nil {
		return fmt.Errorf("the order dispatch: %w", err)
	}
	if drained.Dispatched != 1 {
		return fmt.Errorf("the committed dispatch of the pilot order, got %+v", drained)
	}

	// 8. The verification task of 4-7 over the committed receipt: the
	// assignment (the owner and the backup of the human coverage), the
	// assignee's acceptance and the APPROVED decision over the LIVE order
	// digest (the decision fence re-derives the same composition).
	authView, err := p.order.ReadAuthorizationView(ctx, actor, tenantID, full.CaseID, now.Add(11*time.Minute))
	if err != nil {
		return fmt.Errorf("the authorization view: %w", err)
	}
	var operationID id.UUID
	for _, operation := range authView.Operations {
		if operation.DefinitionID == actionDomain.OrderActionID {
			operationID = operation.OperationID
			break
		}
	}
	if !operationID.Valid() {
		return fmt.Errorf("the pilot order operation not found")
	}
	if _, err := p.order.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, WorkItemID: workItemID,
		OperationID: operationID, Assignee: principalID, BackupOwner: secondOperatorID,
		Deadline: actionDomain.Verify4h, Now: now.Add(12 * time.Minute),
	}); err != nil {
		return fmt.Errorf("the verification assignment: %w", err)
	}
	if _, err := p.order.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID,
		VerificationNo: 1, ExpectedAssignment: 1, Now: now.Add(13 * time.Minute),
	}); err != nil {
		return fmt.Errorf("the verification acceptance: %w", err)
	}
	liveMaterial, err := p.order.ReadOrderVerificationLiveMaterial(ctx, actor, tenantID, full.CaseID)
	if err != nil {
		return fmt.Errorf("the live order material: %w", err)
	}
	if _, err := p.order.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID,
		VerificationNo: 1, ExpectedAssignment: 1,
		ExpectedOrderDigest: actionDomain.OrderDigestOf(liveMaterial), Now: now.Add(14 * time.Minute),
	}); err != nil {
		return fmt.Errorf("the verification approval: %w", err)
	}

	// 9. The supplier facts of 4-9 (the human receipts): the formed and
	// the sent records of the SAME supplier order ID — the
	// sent-after-formed ladder of the full history.
	if _, err := p.order.RecordOrderSupplierFormed(ctx, actionAPI.RecordOrderSupplierFactCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, OperationID: operationID,
		Kind: actionDomain.OrderSupplierFactFormed, SupplierOrderID: pilotSupplierID,
		HappenedAt: now.Add(15 * time.Minute), Now: now.Add(15 * time.Minute),
	}); err != nil {
		return fmt.Errorf("the supplier formed fact: %w", err)
	}
	if _, err := p.order.RecordOrderSupplierSent(ctx, actionAPI.RecordOrderSupplierFactCommand{
		Actor: actor, TenantID: tenantID, CaseID: full.CaseID, OperationID: operationID,
		Kind: actionDomain.OrderSupplierFactSent, SupplierOrderID: pilotSupplierID,
		HappenedAt: now.Add(16 * time.Minute), Now: now.Add(16 * time.Minute),
	}); err != nil {
		return fmt.Errorf("the supplier sent fact: %w", err)
	}

	// 10. The SECOND FRESH case: the inquiry-only row of the honest
	// funnel — the accepted inbound event and NOTHING else (the honest
	// not_reached ladder the outcome panel speaks).
	fresh, err := p.ingestPilotCase(ctx, pilotFreshEventID, pilotFreshSession,
		`{"say":"Здравствуйте, подскажите наличие детали на мою машину"}`, now.Add(time.Minute))
	if err != nil {
		return err
	}
	p.caseID, p.freshCaseID, p.seeded = full.CaseID, fresh.CaseID, true
	return nil
}

// ingestPilotCase ingests ONE accepted inbound channel event of the chat
// connection of the offers world (the journey-offers session prefix pins
// the 6.0.0 pack — the schema-bearing world of the order family) and
// returns the accepted result (the case, the conversation and the event
// anchors). The UNIQUE provider event id keeps the seed collision-free
// (the seed register of the order family).
func (p *pilotOutcomeFixture) ingestPilotCase(ctx context.Context, eventID, sessionID, payload string, now time.Time) (casesAPI.IngestResult, error) {
	command := casesAPI.IngestCommand{
		Envelope: casesDomain.Envelope{
			SchemaVersion: 1, TenantClaim: tenantID, ConnectionID: fixtureChatConnectionID,
			Provider: "staffy-synthetic-telephony", ProviderEventID: eventID, ProviderSessionID: sessionID,
			Direction: casesDomain.DirectionInbound, Modality: "text", Channel: "web_chat",
			ProviderTimestamp: now.Add(-time.Minute),
			Payload:           []byte(payload),
			EnvelopeBytes:     256,
		},
		IdempotencyKey: "pilot-outcome-" + eventID,
		CorrelationID:  id.MustParse("01994f64-6c00-7000-8000-000000000748"),
		Now:            now,
	}
	outcome, err := p.cases.IngestEvent(ctx, command)
	if err != nil {
		return casesAPI.IngestResult{}, err
	}
	if outcome.Accepted == nil {
		refusal := "unknown refusal"
		if outcome.Refused != nil {
			refusal = outcome.Refused.Kind + ": " + outcome.Refused.Message
		}
		if outcome.Quarantined != nil {
			refusal = "quarantine " + outcome.Quarantined.Kind
		}
		return casesAPI.IngestResult{}, fmt.Errorf("the pilot case ingest of %s refused: %s", eventID, refusal)
	}
	return *outcome.Accepted, nil
}

// ensureEntitlement walks the LIVE entitlement admission of the order
// machine (the z47 §0 walk, server-side): the active contour admits, the
// paused one resumes, every other state composes the terms and activates
// — the REAL commands of the entitlement fixture, nothing invented.
func (p *pilotOutcomeFixture) ensureEntitlement(ctx context.Context, actor identityDomain.Principal) error {
	view, err := p.entitlement.EntitlementView(ctx, actor, tenantID)
	if err != nil {
		return fmt.Errorf("the entitlement view: %w", err)
	}
	switch view.State {
	case entitlementDomain.StateActive:
		return nil
	case entitlementDomain.StatePaused:
		if _, err := p.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
			Actor: actor, TenantID: tenantID, Action: entitlementDomain.EventResume,
			Basis: "Возобновление для отчёта пилота автозапчастей", Confirm: true,
		}); err != nil {
			return fmt.Errorf("the entitlement resume: %w", err)
		}
		return nil
	}
	periodStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	launch, subscription := 49000.0, 15000.0
	if _, err := p.entitlement.ComposeTerms(ctx, entitlementAPI.TermsCommand{
		Actor: actor, TenantID: tenantID, Currency: "RUB", TaxRegime: "УСН",
		PeriodStart: &periodStart, PeriodEnd: &periodEnd,
		IncludedEmployees:  []string{"consultant-01"},
		IncludedChannels:   []string{"jivo"},
		IncludedConnectors: []string{"abcp"},
		LaunchPrice:        &launch, SubscriptionPrice: &subscription,
		ChangeNote: "Коммерческое предложение пилота автозапчастей",
	}); err != nil {
		return fmt.Errorf("the entitlement terms: %w", err)
	}
	if _, err := p.entitlement.RunStateTransition(ctx, entitlementAPI.StateCommand{
		Actor: actor, TenantID: tenantID, Action: entitlementDomain.EventActivate,
		Basis: "Договор №86 от 01.10.2026", Confirm: true,
	}); err != nil {
		return fmt.Errorf("the entitlement activation: %w", err)
	}
	return nil
}

// anchors returns the two seeded case ids (the mutex-guarded read of the
// endpoint response — the serial journeys world keeps them stable).
func (p *pilotOutcomeFixture) anchors() (caseID, freshCaseID id.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.caseID, p.freshCaseID
}

// reset routes the action surfaces off the order world (the
// /fixture/order-world family reset of the journey finally blocks — the
// seeded history stays append-only).
func (p *pilotOutcomeFixture) reset() {
	p.actions.setOrder(false)
}

// intPtr is the honest pointer helper of the fixture seeding (the term
// days word of the offer).
func intPtr(value int) *int {
	return &value
}
