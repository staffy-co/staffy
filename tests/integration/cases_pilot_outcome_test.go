//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ---------------------------------------------------------------------------
// Story 4.11 (step-04, part A): the pilot outcome eval matrix — the I/O &
// Edge-Case Matrix rows LIVE over the recorded world of 4-2..4-9 (the
// READ-ONLY derivation of the ten-stage inquiry funnel, the attribution of
// the first accepted inbound channel event, the late-fact flip and the
// aggregate period report — ZERO writes, ZERO migrations). Every row owns
// its own case (the order family precedent: the per-row 4-2..4-5 chain,
// the builder proposal, the confirmation, the authorization, the committed
// dispatch and the verification decision walk the helpers of the 4-6..4-10
// matrices). The world pins casesClock = 2026-10-09 12:00 UTC: every case
// of the matrix is created in October 2026 EXCEPT the out-of-period row
// (the ONE time-control adaptation of the matrix — the explicit ingest Now
// of the November case, the created_at the honest period filter reads).
// ---------------------------------------------------------------------------

// pilotReadNow is the read clock of the matrix (the derivation reads the
// RECORDED times only — the parameter stays for the surface symmetry of
// the read family, the same world answers the same).
var pilotReadNow = orderNow.Add(90 * time.Minute)

// pilotWorld wires the order world with the pilot outcome contour (the
// same service of the 4-6..4-10 matrices — the WithPilotOutcome wiring of
// buildOrderService; the pilot port is the LIVE cases service).
func pilotWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, identityDomain.Principal, identityDomain.Principal) {
	t.Helper()
	w, svc, adapter, _, _, operator := orderWorld(t, seed)
	backup := identityDomain.Principal{ID: w.backup}
	return w, svc, adapter, operator, backup
}

// pilotFreshCase ingests ONE accepted inbound channel event of the world
// connection and returns the fresh case (the attribution material of the
// outcome is the ingested event itself — only what the source carried).
func pilotFreshCase(t *testing.T, w *casesWorld, tag string) id.UUID {
	t.Helper()
	ingest, err := w.cases.IngestEvent(context.Background(),
		w.ingestCommand("evt-pilot-"+tag, "sess-pilot-"+tag, `{"call":"inbound-call","from":"+79000000796"}`))
	if err != nil {
		t.Fatal(err)
	}
	return acceptedOutcome(t, ingest).CaseID
}

// pilotCommittedOrder walks one row's case to the COMMITTED order receipt
// with the EXPLICIT external ABCP order ID (the shared-ID rows of the
// double-count probe — the anchor stays the operation of the case, never
// the external ID; the verificationCommittedOrder walk with the external
// ID taken by the parameter).
func pilotCommittedOrder(t *testing.T, w *casesWorld, svc *actionApp.Service, adapter *countingMutationAdapter,
	operator identityDomain.Principal, tag, key, externalID string) (id.UUID, id.UUID, actionDomain.Operation) {
	t.Helper()
	caseID, workItemID := orderRowCase(t, w, tag)
	_, _, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, key, tag)
	// The synthetic dispatch: committed with the external triple.
	adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "corr-pilot-"+tag, nil)
	adapter.inner.ScriptEvidence(externalID, "3")
	adapter.reset()
	report, err := svc.DispatchQueuedOperations(context.Background(), actionApp.DispatchDrainCommand{
		Actor: operator, TenantID: w.tenant, Now: orderNow.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Dispatched != 1 {
		t.Fatalf("the committed dispatch of the row %q, got %+v", tag, report)
	}
	return caseID, workItemID, operation
}

// pilotOutcome reads the derived outcome of one case.
func pilotOutcome(t *testing.T, svc *actionApp.Service, actor identityDomain.Principal, w *casesWorld, caseID id.UUID) actionDomain.PilotCaseOutcome {
	t.Helper()
	outcome, err := svc.ReadPilotCaseOutcome(context.Background(), actor, w.tenant, caseID, pilotReadNow)
	if err != nil {
		t.Fatalf("the outcome read of the case %s refused: %v", caseID, err)
	}
	return outcome
}

// pilotReport reads the derived period report of the tenant.
func pilotReport(t *testing.T, svc *actionApp.Service, actor identityDomain.Principal, w *casesWorld, period string) actionDomain.PilotReport {
	t.Helper()
	report, err := svc.ReadPilotReport(context.Background(), actor, w.tenant, period, pilotReadNow)
	if err != nil {
		t.Fatalf("the pilot report read of the period %q refused: %v", period, err)
	}
	return report
}

// pilotStageOf finds one stage evidence of the outcome (BY NAME).
func pilotStageOf(t *testing.T, outcome actionDomain.PilotCaseOutcome, stage string) actionDomain.PilotStageEvidence {
	t.Helper()
	for _, row := range outcome.Stages {
		if row.Stage == stage {
			return row
		}
	}
	t.Fatalf("the outcome must carry the %s stage, got %+v", stage, outcome.Stages)
	return actionDomain.PilotStageEvidence{}
}

// pilotConversionOf finds one conversion of the report (BY NAME).
func pilotConversionOf(t *testing.T, report actionDomain.PilotReport, from, to string) actionDomain.PilotConversion {
	t.Helper()
	for _, row := range report.Conversions {
		if row.From == from && row.To == to {
			return row
		}
	}
	t.Fatalf("the report must carry the %s → %s conversion, got %+v", from, to, report.Conversions)
	return actionDomain.PilotConversion{}
}

// TestPilotOutcomeMatrixRowsBySpec walks the Story 4.11 eval matrix LIVE:
// the fresh case (inquiry proven with the attribution, the honest
// not_reached ladder, the terminal unknown_pending pins OQ-4/OQ-3/№215),
// the vehicle confirmation of 4-2, the applicability proof of 4-4, the
// committed order receipt of 4-6, the approved verification decision of
// 4-7, the sent supplier fact of 4-9, the late-fact flip (AC-13 — the
// read-time re-derivation, the append-only history untouched), the
// double-count impossibility (AC-3/AC-17 — the anchor is the operation of
// the case, never the external ID), the aggregate report (every outcome
// counted, the conversions with numerator/denominator, the honest unknown
// share, the OQ-7 boundary word), the period filter (the out-of-period
// case never counted, the empty period honest), the RLS blindness and the
// scope refusal. The double-count row runs BEFORE the order-machine rows
// 4..7 (documented in the row): at that point only its two cases carry a
// committed receipt, so the report pin StageCounts[client_order_created]
// == 2 stays EXACT — the cleanest structure for the AC-3/AC-17 probe.
func TestPilotOutcomeMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	// The world seed 773: the free range of the order family (757..759
	// and 770..772 are taken — the UNIQUE seed keeps the shared-DB
	// worlds collision-free, the seed register of the family).
	w, svc, adapter, operator, backup := pilotWorld(t, 773)

	// The case of the fresh row rides the later rows (the RLS and the
	// scope probes read the honest material anchor of the world).
	var freshCaseID id.UUID

	// Row 1 — the FRESH CASE: one accepted inbound channel event — the
	// inquiry stage is proven with the attribution of the ingested event
	// {provider, channel, provider_event_id, time}; every measurable
	// stage after it stays not_reached (the material is absent); the
	// four terminal stages are unknown_pending ALWAYS with the pins
	// (OQ-4 on fulfilled/paid, OQ-3 + №215 on cancelled, 3-8 on returned
	// — never zero, never a silent exclusion, AC-14).
	t.Run("row-1 fresh case: inquiry proven with the attribution; the ladder not_reached; the terminals unknown_pending", func(t *testing.T) {
		freshCaseID = pilotFreshCase(t, w, "fresh-1")
		outcome := pilotOutcome(t, svc, operator, w, freshCaseID)
		if !outcome.CaseFound || outcome.CaseNo != 1 {
			t.Fatalf("the first case of the world, got %+v", outcome)
		}
		// The attribution of the FIRST accepted inbound channel event —
		// only what the source carried (AC-1): the campaign/referrer
		// stay the unknown words, never a guess.
		attribution := outcome.Attribution
		if !attribution.Found || attribution.Provider != "staffy-synthetic-telephony" ||
			attribution.Channel != "phone" || attribution.ProviderEventID != "evt-pilot-fresh-1" ||
			!attribution.ReceivedAt.Equal(casesClock.Now()) {
			t.Fatalf("the attribution of the ingested event, got %+v", attribution)
		}
		if attribution.CampaignWord != actionDomain.PilotAttributionUnknownWord ||
			attribution.ReferrerWord != actionDomain.PilotAttributionUnknownWord {
			t.Fatalf("the campaign/referrer unknown words, got %+v", attribution)
		}
		if !strings.Contains(attribution.Word, actionDomain.PilotAttributionProviderWord+" staffy-synthetic-telephony") ||
			!strings.Contains(attribution.Word, actionDomain.PilotAttributionChannelWord+" phone") ||
			!strings.Contains(attribution.Word, actionDomain.PilotAttributionEventIDWord+" evt-pilot-fresh-1") {
			t.Fatalf("the attribution word, got %q", attribution.Word)
		}
		// The inquiry stage: proven with the channel-event source and
		// the case creation time.
		inquiry := pilotStageOf(t, outcome, actionDomain.PilotStageInquiryAccepted)
		if inquiry.State != actionDomain.PilotStageStateProven || inquiry.Evidence != actionDomain.PilotInquiryAcceptedWord ||
			inquiry.SourceWord != actionDomain.PilotSourceChannelEventWord || !inquiry.Time.Equal(casesClock.Now()) {
			t.Fatalf("the proven inquiry stage, got %+v", inquiry)
		}
		// The five measurable stages after the inquiry: not_reached (the
		// honest ladder — the material is absent, no pin words).
		for _, stage := range []string{
			actionDomain.PilotStageVehicleProfileConfirmed, actionDomain.PilotStageApplicabilityProven,
			actionDomain.PilotStageClientOrderCreated, actionDomain.PilotStageManagerVerified,
			actionDomain.PilotStageSupplierSent,
		} {
			row := pilotStageOf(t, outcome, stage)
			if row.State != actionDomain.PilotStageStateNotReached || row.PinWord != "" {
				t.Fatalf("the not_reached stage %s, got %+v", stage, row)
			}
		}
		// The vehicle miss word of the fresh case (the identification
		// material is absent — the inquiry never reached the profile).
		if vehicle := pilotStageOf(t, outcome, actionDomain.PilotStageVehicleProfileConfirmed); vehicle.Evidence != actionDomain.PilotVehicleNotReachedWord {
			t.Fatalf("the vehicle miss word, got %q", vehicle.Evidence)
		}
		// The four terminal stages: unknown_pending ALWAYS with the pins
		// BY NAME (the owning sources live outside the contour).
		for _, pin := range []struct {
			stage string
			word  string
		}{
			{actionDomain.PilotStageFulfilled, actionDomain.PilotFulfilledPinWord},
			{actionDomain.PilotStagePaid, actionDomain.PilotPaidPinWord},
			{actionDomain.PilotStageCancelled, actionDomain.PilotCancelledPinWord},
			{actionDomain.PilotStageReturned, actionDomain.PilotReturnedPinWord},
		} {
			row := pilotStageOf(t, outcome, pin.stage)
			if row.State != actionDomain.PilotStageStateUnknownPending || row.PinWord != pin.word ||
				row.StateWord != actionDomain.PilotStageStateWords[actionDomain.PilotStageStateUnknownPending] {
				t.Fatalf("the unknown_pending terminal stage %s, got %+v", pin.stage, row)
			}
		}
		if !strings.Contains(actionDomain.PilotFulfilledPinWord, "OQ-4") || !strings.Contains(actionDomain.PilotPaidPinWord, "OQ-4") ||
			!strings.Contains(actionDomain.PilotCancelledPinWord, "OQ-3") || !strings.Contains(actionDomain.PilotCancelledPinWord, "№215") {
			t.Fatal("the terminal pin words must carry the OQ-4/OQ-3/№215 pins")
		}
		// The ten stages arrive in the dictionary order.
		if !actionDomain.PilotStageEvidenceSortedByDictionary(outcome.Stages) || len(outcome.Stages) != 10 {
			t.Fatalf("the ten stages in the dictionary order, got %+v", outcome.Stages)
		}
		// The counters of the fresh case: nothing recorded yet.
		if outcome.CorrectionsCount != 0 || outcome.HandoffCount != 0 || outcome.RecoveryCount != 0 {
			t.Fatalf("the fresh case counters, got %+v", outcome)
		}
	})

	// Row 2 — the VEHICLE CONFIRMED (the 4-2 material): the
	// identification facts land through the live operator commands and
	// the operator confirms the exact version (the vehicle matrix row-3
	// path — confirmer=operator_relay) — the stage is proven with the
	// confirmed keys riding the evidence; the applicability stays
	// not_reached (no fitment material yet).
	t.Run("row-2 vehicle confirmed: the confirmed keys ride the evidence; the applicability stays not_reached", func(t *testing.T) {
		caseID := pilotFreshCase(t, w, "vehicle-2")
		detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		workItemID := detail.WorkItems[0].WorkItemID
		vin := vehicleRecordFact(t, w, caseID, workItemID, "vehicle.vin", []byte(`" WF0XXXTT4S1234567 "`), "pilot-vehicle-vin-2")
		vehicleRecordFact(t, w, caseID, workItemID, "vehicle.make", []byte(`"Ford"`), "pilot-vehicle-make-2")
		vehicleRecordFact(t, w, caseID, workItemID, "vehicle.model", []byte(`"Focus"`), "pilot-vehicle-model-2")
		vehicleRecordFact(t, w, caseID, workItemID, "vehicle.year_of_manufacture", []byte(`"2021"`), "pilot-vehicle-year-2")
		if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: vin.FactID,
			VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
			Note: "Клиент продиктовал VIN в разговоре", IdempotencyKey: "pilot-vehicle-confirm-vin-2", Now: casesClock.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		outcome := pilotOutcome(t, svc, operator, w, caseID)
		vehicle := pilotStageOf(t, outcome, actionDomain.PilotStageVehicleProfileConfirmed)
		if vehicle.State != actionDomain.PilotStageStateProven || vehicle.SourceWord != actionDomain.PilotSourceVehicleConfirmationWord {
			t.Fatalf("the proven vehicle stage, got %+v", vehicle)
		}
		if !strings.Contains(vehicle.Evidence, actionDomain.PilotVehicleConfirmedWord) || !strings.Contains(vehicle.Evidence, "vehicle.vin") {
			t.Fatalf("the confirmed keys ride the vehicle evidence, got %q", vehicle.Evidence)
		}
		if vehicle.Time.IsZero() {
			t.Fatal("the vehicle confirmation time is zero")
		}
		// The applicability stays not_reached (the spec edge: the
		// profile confirmation never proves the fitment).
		applicability := pilotStageOf(t, outcome, actionDomain.PilotStageApplicabilityProven)
		if applicability.State != actionDomain.PilotStageStateNotReached || applicability.Evidence != actionDomain.PilotFitmentNotReachedWord {
			t.Fatalf("the applicability without the fitment material, got %+v", applicability)
		}
	})

	// Row 3 — the APPLICABILITY PROVEN (the 4-4 material): the confirmed
	// part request fact (the request pin of the fitment matrix row-2
	// path) plus the found original fitment record — the stage is proven
	// with the position riding the evidence; the order stage stays
	// not_reached (no operation of the case).
	t.Run("row-3 applicability proven: the fitment evidence of 4-4; the order stays not_reached", func(t *testing.T) {
		caseID := pilotFreshCase(t, w, "fitment-3")
		detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		workItemID := detail.WorkItems[0].WorkItemID
		kind := partRecordFact(t, w, caseID, workItemID, "part.kind", []byte(`"фильтр салона"`), "pilot-fitment-kind-3")
		if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: kind.FactID,
			VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
			Note: "Клиент назвал деталь в разговоре", IdempotencyKey: "pilot-fitment-confirm-kind-3", Now: casesClock.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		fitmentRecord(t, w, caseID, workItemID, "pilot-3", nil)
		outcome := pilotOutcome(t, svc, operator, w, caseID)
		applicability := pilotStageOf(t, outcome, actionDomain.PilotStageApplicabilityProven)
		if applicability.State != actionDomain.PilotStageStateProven || applicability.SourceWord != actionDomain.PilotSourceFitmentEvidenceWord {
			t.Fatalf("the proven applicability stage, got %+v", applicability)
		}
		if !strings.Contains(applicability.Evidence, actionDomain.PilotApplicabilityProvenWord) || !strings.Contains(applicability.Evidence, "WF2B196K666AA") {
			t.Fatalf("the fitment position rides the applicability evidence, got %q", applicability.Evidence)
		}
		if applicability.Time.IsZero() {
			t.Fatal("the fitment evidence time is zero")
		}
		// The order stage stays not_reached (no operation exists).
		order := pilotStageOf(t, outcome, actionDomain.PilotStageClientOrderCreated)
		if order.State != actionDomain.PilotStageStateNotReached || order.Evidence != actionDomain.PilotOrderNotReachedWord {
			t.Fatalf("the order miss without an operation, got %+v", order)
		}
	})

	// Row 8 — the DOUBLE-COUNT IMPOSSIBILITY (AC-3/AC-17). THE ROW RUNS
	// BEFORE the order-machine rows 4..7 (the documented structure): at
	// this point only the five cases of the rows 1..3 and THIS row exist
	// and only the two cases of this row carry a committed receipt — the
	// report pin StageCounts[client_order_created] == 2 stays EXACT. Two
	// DIFFERENT cases with the SAME external ABCP order ID still count
	// EACH case separately: the anchor is the operation of the case, the
	// report counts CASES, never external results.
	t.Run("row-8 double-count impossibility: one external ID, two cases, two operations — the report counts the cases", func(t *testing.T) {
		const sharedExternalID = "ABCP-ORDER-PILOT-DUP"
		caseA, _, operationA := pilotCommittedOrder(t, w, svc, adapter, operator, "p8a", fmtOrderTarget(966), sharedExternalID)
		caseB, _, operationB := pilotCommittedOrder(t, w, svc, adapter, operator, "p8b", fmtOrderTarget(967), sharedExternalID)
		if operationA.OperationID == operationB.OperationID {
			t.Fatal("the two cases must anchor TWO DIFFERENT operations")
		}
		outcomeA := pilotOutcome(t, svc, operator, w, caseA)
		outcomeB := pilotOutcome(t, svc, operator, w, caseB)
		createdA := pilotStageOf(t, outcomeA, actionDomain.PilotStageClientOrderCreated)
		createdB := pilotStageOf(t, outcomeB, actionDomain.PilotStageClientOrderCreated)
		if createdA.State != actionDomain.PilotStageStateProven || createdB.State != actionDomain.PilotStageStateProven {
			t.Fatalf("both cases prove the order creation, got %+v / %+v", createdA, createdB)
		}
		// The SAME external ID rides both evidences (the divergent
		// external IDs material of 4-9/4-10 — the anchor is NEVER the
		// external ID).
		for _, evidence := range []string{createdA.Evidence, createdB.Evidence} {
			if !strings.Contains(evidence, actionDomain.PilotClientOrderCreatedWord) || !strings.Contains(evidence, sharedExternalID) {
				t.Fatalf("the shared external ID rides the evidence, got %q", evidence)
			}
		}
		// The report over the five first cases of the world: TWO cases
		// with proven order creation — counting the external IDs would
		// answer ONE (the double count of one inquiry is impossible
		// structurally).
		report := pilotReport(t, svc, operator, w, "2026-10")
		if report.CaseVolume != 5 || report.SampleSize != 5 {
			t.Fatalf("the five first cases of the world, got %+v", report)
		}
		if report.StageCounts[actionDomain.PilotStageClientOrderCreated] != 2 {
			t.Fatalf("the two cases with the SAME external ID count as TWO, got %d",
				report.StageCounts[actionDomain.PilotStageClientOrderCreated])
		}
		if report.StageCounts[actionDomain.PilotStageApplicabilityProven] != 3 {
			t.Fatalf("the applicability of the five first cases, got %d",
				report.StageCounts[actionDomain.PilotStageApplicabilityProven])
		}
	})

	// Row 4 — the ORDER CREATED (the 4-6 material): the order operation
	// with the committed receipt carrying the external ABCP order ID —
	// the stage is proven with the external ID riding the evidence; the
	// manager verification stays not_reached (no closed decision).
	t.Run("row-4 order created: the committed receipt carries the external ID; the verification stays not_reached", func(t *testing.T) {
		caseID, _, _, _ := verificationCommittedOrder(t, w, svc, adapter, operator, "p4", fmtOrderTarget(961))
		outcome := pilotOutcome(t, svc, operator, w, caseID)
		created := pilotStageOf(t, outcome, actionDomain.PilotStageClientOrderCreated)
		if created.State != actionDomain.PilotStageStateProven || created.SourceWord != actionDomain.PilotSourceOperationReceiptWord {
			t.Fatalf("the proven order stage, got %+v", created)
		}
		if !strings.Contains(created.Evidence, actionDomain.PilotClientOrderCreatedWord) || !strings.Contains(created.Evidence, "ABCP-ORDER-p4") {
			t.Fatalf("the external ID rides the order evidence, got %q", created.Evidence)
		}
		if created.Time.IsZero() {
			t.Fatal("the receipt time of the order stage is zero")
		}
		// The manager verification stays not_reached (the spec edge: the
		// created order never proves the verification).
		verified := pilotStageOf(t, outcome, actionDomain.PilotStageManagerVerified)
		if verified.State != actionDomain.PilotStageStateNotReached || verified.Evidence != actionDomain.PilotVerifiedNotReachedWord {
			t.Fatalf("the verification miss without a decision, got %+v", verified)
		}
	})

	// Row 5 — the MANAGER VERIFIED (the 4-7 material): the verification
	// task with the approved closed decision — the stage is proven with
	// the decision evidence; the supplier sent stays not_reached (no
	// recorded fact).
	t.Run("row-5 manager verified: the approved closed decision proves the stage", func(t *testing.T) {
		caseID, _, _ := statusApprovedOrder(t, w, svc, adapter, operator, backup, "p5", fmtOrderTarget(962))
		outcome := pilotOutcome(t, svc, operator, w, caseID)
		verified := pilotStageOf(t, outcome, actionDomain.PilotStageManagerVerified)
		if verified.State != actionDomain.PilotStageStateProven || verified.Evidence != actionDomain.PilotManagerVerifiedWord ||
			verified.SourceWord != actionDomain.PilotSourceVerificationDecisionWord || verified.Time.IsZero() {
			t.Fatalf("the proven verification stage, got %+v", verified)
		}
		// The supplier sent stays not_reached (no fact recorded).
		sent := pilotStageOf(t, outcome, actionDomain.PilotStageSupplierSent)
		if sent.State != actionDomain.PilotStageStateNotReached || sent.Evidence != actionDomain.PilotSentNotReachedWord {
			t.Fatalf("the sent miss without a fact, got %+v", sent)
		}
	})

	// Row 6 — the SUPPLIER SENT (the 4-9 record command path): the formed
	// and sent facts of the SAME supplier order ID — the stage is proven
	// with the supplier order ID riding the evidence and the happened
	// time of the earliest sent fact as the stage time.
	t.Run("row-6 supplier sent: the recorded sent fact proves the stage with the supplier order ID", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "p6", fmtOrderTarget(963))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-P6",
			orderNow.Add(30*time.Minute), orderNow.Add(31*time.Minute))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactSent, "SUP-P6",
			orderNow.Add(40*time.Minute), orderNow.Add(41*time.Minute))
		outcome := pilotOutcome(t, svc, operator, w, caseID)
		sent := pilotStageOf(t, outcome, actionDomain.PilotStageSupplierSent)
		if sent.State != actionDomain.PilotStageStateProven || sent.SourceWord != actionDomain.PilotSourceSupplierFactWord {
			t.Fatalf("the proven sent stage, got %+v", sent)
		}
		if !strings.Contains(sent.Evidence, actionDomain.PilotSupplierSentWord) || !strings.Contains(sent.Evidence, "SUP-P6") {
			t.Fatalf("the supplier order ID rides the sent evidence, got %q", sent.Evidence)
		}
		if !sent.Time.Equal(orderNow.Add(40 * time.Minute)) {
			t.Fatalf("the sent stage time is the happened time of the sent fact, got %v", sent.Time)
		}
	})

	// Row 7 — the LATE-FACT FLIP (AC-13): the outcome is read BEFORE the
	// sent fact (the stage is NEVER claimed — not_reached with the
	// honest miss word), the manager records the formed and sent facts,
	// the outcome is read AGAIN — the read-time re-derivation FLIPS the
	// stage to proven WITHOUT rewriting anything (the append-only
	// history backstop: exactly two fact rows; the repeated read of the
	// same world answers the same).
	t.Run("row-7 late-fact flip: the read BEFORE claims nothing; the read AFTER flips; the history stays append-only", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "p7", fmtOrderTarget(964))
		// The honest read BEFORE the late fact: the sent stage is never
		// claimed (the false send claim is impossible structurally).
		before := pilotOutcome(t, svc, operator, w, caseID)
		sentBefore := pilotStageOf(t, before, actionDomain.PilotStageSupplierSent)
		if sentBefore.State != actionDomain.PilotStageStateNotReached || sentBefore.Evidence != actionDomain.PilotSentNotReachedWord {
			t.Fatalf("the sent stage BEFORE the late fact, got %+v", sentBefore)
		}
		// The late facts of the manager (the 4-9 record commands): the
		// formed then sent records of the SAME supplier order ID.
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-P7",
			orderNow.Add(30*time.Minute), orderNow.Add(31*time.Minute))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactSent, "SUP-P7",
			orderNow.Add(40*time.Minute), orderNow.Add(41*time.Minute))
		// The read AGAIN: the read-time re-derivation flips the stage
		// (AC-13 — nothing was rewritten, the world was only appended).
		after := pilotOutcome(t, svc, operator, w, caseID)
		sentAfter := pilotStageOf(t, after, actionDomain.PilotStageSupplierSent)
		if sentAfter.State != actionDomain.PilotStageStateProven || !strings.Contains(sentAfter.Evidence, "SUP-P7") {
			t.Fatalf("the sent stage AFTER the late fact, got %+v", sentAfter)
		}
		// The repeated read of the same world answers the SAME (the
		// idempotent read — the same world, the same answer).
		repeat := pilotOutcome(t, svc, operator, w, caseID)
		if repeatSent := pilotStageOf(t, repeat, actionDomain.PilotStageSupplierSent); repeatSent.State != actionDomain.PilotStageStateProven {
			t.Fatalf("the repeated read flips nothing, got %+v", repeatSent)
		}
		// The append-only backstop: exactly TWO fact rows (formed then
		// sent) — the reads NEVER rewrote the history.
		rows := statusFactRows(t, w.tenant, caseID)
		if len(rows) != 2 || rows[0].Kind != actionDomain.OrderSupplierFactFormed || rows[1].Kind != actionDomain.OrderSupplierFactSent {
			t.Fatalf("the append-only fact rows after the flip, got %+v", rows)
		}
	})

	// Row 9 — the REPORT AGGREGATE: the period report over the whole
	// world of the rows (NINE October cases): the case volume with the
	// sample size and the period word, every stage counted BY NAME, the
	// conversions between the PROVEN neighbours with the
	// numerator/denominator BY NAME, the honest unknown share (the
	// terminals are never proven — the numerator equals the denominator
	// equals the case count), the OQ-7 boundary word and the Measurement
	// Dictionary v1.
	t.Run("row-9 report aggregate: every outcome counted; the conversions carry numerator/denominator; the unknown share is honest", func(t *testing.T) {
		report := pilotReport(t, svc, operator, w, "2026-10")
		if report.CaseVolume != 9 || report.SampleSize != 9 {
			t.Fatalf("the nine October cases of the matrix, got %+v", report)
		}
		if report.PeriodWord != "период: 2026-10" {
			t.Fatalf("the period word of the report, got %q", report.PeriodWord)
		}
		if report.EmptySampleWord != "" {
			t.Fatalf("the non-empty sample carries no empty word, got %q", report.EmptySampleWord)
		}
		// Every outcome counted BY NAME (the world: one vehicle row, one
		// applicability-only row, six order rows with the fitment chain,
		// three approved verifications, two sent facts).
		expected := map[string]int{
			actionDomain.PilotStageInquiryAccepted:         9,
			actionDomain.PilotStageVehicleProfileConfirmed: 1,
			actionDomain.PilotStageApplicabilityProven:     7,
			actionDomain.PilotStageClientOrderCreated:      6,
			actionDomain.PilotStageManagerVerified:         3,
			actionDomain.PilotStageSupplierSent:            2,
			actionDomain.PilotStageFulfilled:               0,
			actionDomain.PilotStagePaid:                    0,
			actionDomain.PilotStageCancelled:               0,
			actionDomain.PilotStageReturned:                0,
		}
		for stage, count := range expected {
			if report.StageCounts[stage] != count {
				t.Fatalf("the stage count of %s, got %d want %d", stage, report.StageCounts[stage], count)
			}
		}
		// The conversions between the ADJACENT proven stages carry the
		// numerator/denominator BY NAME (the unknown_pending stages
		// never enter a denominator — the honest measurability fence).
		for _, conversion := range []struct {
			from        string
			to          string
			numerator   int
			denominator int
		}{
			{actionDomain.PilotStageInquiryAccepted, actionDomain.PilotStageVehicleProfileConfirmed, 1, 9},
			{actionDomain.PilotStageVehicleProfileConfirmed, actionDomain.PilotStageApplicabilityProven, 0, 1},
			{actionDomain.PilotStageApplicabilityProven, actionDomain.PilotStageClientOrderCreated, 6, 7},
			{actionDomain.PilotStageClientOrderCreated, actionDomain.PilotStageManagerVerified, 3, 6},
			{actionDomain.PilotStageManagerVerified, actionDomain.PilotStageSupplierSent, 2, 3},
		} {
			row := pilotConversionOf(t, report, conversion.from, conversion.to)
			if row.Numerator != conversion.numerator || row.Denominator != conversion.denominator {
				t.Fatalf("the conversion %s → %s, got %d/%d want %d/%d", conversion.from, conversion.to,
					row.Numerator, row.Denominator, conversion.numerator, conversion.denominator)
			}
		}
		// The honest unknown share: the terminals are never proven in
		// this slice — the numerator equals the denominator equals the
		// case count (AC-14 — the influence of the unknown is visible).
		if report.UnknownShare.Numerator != 9 || report.UnknownShare.Denominator != 9 {
			t.Fatalf("the honest unknown share 9/9, got %+v", report.UnknownShare)
		}
		if !strings.Contains(report.UnknownShare.Word, actionDomain.PilotUnknownShareWord) {
			t.Fatalf("the unknown share word, got %q", report.UnknownShare.Word)
		}
		// The counters of the world: no corrections, no handoff
		// requests, no recovery cases were recorded.
		if report.CorrectionsCount != 0 || report.HandoffCount != 0 || report.RecoveryCount != 0 {
			t.Fatalf("the counters of the world, got %+v", report)
		}
		// The OQ-7 boundary word rides the aggregate (AC-12) and the
		// Measurement Dictionary v1 with the four non-measurable rows
		// (AC-9 — the words, never the numbers).
		if report.BoundaryWord != actionDomain.PilotOQ7BoundaryWord {
			t.Fatalf("the OQ-7 boundary word, got %q", report.BoundaryWord)
		}
		if len(report.Dictionary) == 0 || len(report.NotMeasurable) != 4 {
			t.Fatalf("the dictionary of the report, got %d measurable / %d non-measurable rows",
				len(report.Dictionary), len(report.NotMeasurable))
		}
	})

	// Row 10 — the PERIOD FILTER: a case created in a DIFFERENT month
	// (the ONE time-control adaptation of the matrix — the explicit
	// ingest Now of the November case; the harness fixes every other
	// case at casesClock = 2026-10-09 12:00 UTC) is NOT counted by the
	// October report; the November report keeps ONLY the November case;
	// the empty period answers the honest zero sample with the word; the
	// absent period is the whole observation period.
	t.Run("row-10 period filter: the out-of-period case is not counted; the empty period is honest", func(t *testing.T) {
		november := casesClock.Now().AddDate(0, 1, 0)
		command := w.ingestCommand("evt-pilot-nov-10", "sess-pilot-nov-10", `{"call":"inbound-call","from":"+79000000797"}`)
		command.Envelope.ProviderTimestamp = november.Add(-time.Minute)
		command.Now = november
		ingest, err := w.cases.IngestEvent(ctx, command)
		if err != nil {
			t.Fatal(err)
		}
		novemberCase := acceptedOutcome(t, ingest).CaseID
		// The November case reads its own honest outcome (the inquiry is
		// proven with the November attribution of the ingested event).
		outcome := pilotOutcome(t, svc, operator, w, novemberCase)
		inquiry := pilotStageOf(t, outcome, actionDomain.PilotStageInquiryAccepted)
		if inquiry.State != actionDomain.PilotStageStateProven {
			t.Fatalf("the November inquiry stage, got %+v", inquiry)
		}
		if outcome.Attribution.ProviderEventID != "evt-pilot-nov-10" || !outcome.Attribution.ReceivedAt.Equal(november) {
			t.Fatalf("the November attribution, got %+v", outcome.Attribution)
		}
		// The October report: the out-of-period case is NOT counted (the
		// volume stays the nine October cases).
		october := pilotReport(t, svc, operator, w, "2026-10")
		if october.CaseVolume != 9 {
			t.Fatalf("the October report keeps the nine October cases, got %d", october.CaseVolume)
		}
		// The November report: ONLY the November case (the filter keeps
		// the month's cases — never everything).
		novemberReport := pilotReport(t, svc, operator, w, "2026-11")
		if novemberReport.CaseVolume != 1 || novemberReport.SampleSize != 1 || novemberReport.PeriodWord != "период: 2026-11" {
			t.Fatalf("the November report, got %+v", novemberReport)
		}
		if novemberReport.StageCounts[actionDomain.PilotStageInquiryAccepted] != 1 {
			t.Fatalf("the November inquiry count, got %d", novemberReport.StageCounts[actionDomain.PilotStageInquiryAccepted])
		}
		// The empty period: the honest zero sample with the word (the
		// zero is allowed ONLY as the case count) — no conversions are
		// derived over an empty sample.
		empty := pilotReport(t, svc, operator, w, "2026-09")
		if empty.CaseVolume != 0 || empty.EmptySampleWord != actionDomain.PilotEmptySampleWord {
			t.Fatalf("the honest empty period, got %+v", empty)
		}
		if empty.UnknownShare.Numerator != 0 || empty.UnknownShare.Denominator != 0 || len(empty.Conversions) != 0 {
			t.Fatalf("the empty period carries no derived numbers, got %+v", empty)
		}
		// The absent period: the whole observation period — every case
		// of the tenant rides the report (the period word speaks it).
		whole := pilotReport(t, svc, operator, w, "")
		if whole.CaseVolume != 10 || whole.PeriodWord != "весь период наблюдения" {
			t.Fatalf("the whole observation period, got %+v", whole)
		}
	})

	// Row 11 — the RLS BLINDNESS: the foreign tenant's case answers the
	// honest not-found through the outcome surface (both directions) and
	// the foreign tenant's report counts ONLY its own cases (our ten
	// cases never leak into the foreign aggregate — the store RLS
	// mirror); our actor is NOT a member of the foreign tenant — the
	// honest 404 BEFORE any outcome material.
	t.Run("row-11 RLS blindness: the foreign case answers the honest not-found; the foreign report counts only its own", func(t *testing.T) {
		// The foreign world of the RLS row (the 4-6 row-10 precedent —
		// the seed 774 of the free order-family range).
		foreign := newCasesWorld(t, 774)
		foreignIngest, err := foreign.cases.IngestEvent(ctx,
			foreign.ingestCommand("evt-pilot-foreign-11", "sess-pilot-foreign-11", `{"call":"inbound-call","from":"+79000000699"}`))
		if err != nil {
			t.Fatal(err)
		}
		foreignCase := acceptedOutcome(t, foreignIngest).CaseID
		// OUR actor reads the FOREIGN case through the outcome surface:
		// the RLS-blind honest not-found (never an enumeration).
		if _, err := svc.ReadPilotCaseOutcome(ctx, operator, w.tenant, foreignCase, pilotReadNow); !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the foreign case read, got %v", err)
		}
		// The FOREIGN actor reads OUR case: the same honest not-found
		// (the foreign membership passes the gate, the store RLS mirror
		// answers the blindness).
		if _, err := svc.ReadPilotCaseOutcome(ctx, foreign.workActor(), foreign.tenant, freshCaseID, pilotReadNow); !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the foreign actor read of our case, got %v", err)
		}
		// OUR actor is NOT a member of the foreign tenant: the honest
		// 404 BEFORE any outcome material (the tenant existence is
		// never disclosed).
		if _, err := svc.ReadPilotCaseOutcome(ctx, operator, foreign.tenant, freshCaseID, pilotReadNow); !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the non-member read of the foreign tenant, got %v", err)
		}
		// The FOREIGN report counts ONLY the foreign tenant's own case —
		// our ten cases never leak into the foreign aggregate.
		foreignReport := pilotReport(t, svc, foreign.workActor(), foreign, "2026-10")
		if foreignReport.CaseVolume != 1 {
			t.Fatalf("the foreign report counts only its own case, got %d", foreignReport.CaseVolume)
		}
	})

	// Row 12 — the SCOPE REFUSAL: a member WITHOUT the review scope
	// reads NEITHER the outcome panel NOR the pilot report — the honest
	// forbidden error with the scope word BEFORE any outcome material
	// (the review scope is the operator surface family gate).
	t.Run("row-12 scope refusal: the member without the review scope reads nothing", func(t *testing.T) {
		// The configuration-only member (the cases matrix precedent:
		// the case metadata stays visible to the configuration member —
		// the PILOT contour demands the review scope of the family).
		configOnly := inviteMember(t, w.identity, w.owner, w.tenant, "pilot-config-773@example.org", identityDomain.Configurator,
			[]identityDomain.Scope{identityDomain.Configuration}, "000000773101", "000000773102")
		if _, err := svc.ReadPilotCaseOutcome(ctx, configOnly, w.tenant, freshCaseID, pilotReadNow); !errors.Is(err, actionDomain.ErrForbidden) || !strings.Contains(err.Error(), "полномочия review") {
			t.Fatalf("the scope refusal of the outcome panel, got %v", err)
		}
		if _, err := svc.ReadPilotReport(ctx, configOnly, w.tenant, "2026-10", pilotReadNow); !errors.Is(err, actionDomain.ErrForbidden) || !strings.Contains(err.Error(), "полномочия review") {
			t.Fatalf("the scope refusal of the pilot report, got %v", err)
		}
	})
}
