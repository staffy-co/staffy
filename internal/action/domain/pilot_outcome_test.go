package domain

import (
	"reflect"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 4.11 unit matrix of the Pilot Business Outcome domain: the
// dictionaries BY NAME, the per-case derivation (the fresh case, the
// full journey, the late fact flip, the attribution unknowns), the
// aggregate fold (the counts, the proven-only conversions, the honest
// share) and the STRUCTURAL honesty probes (the non-measurable rows
// carry NO numeric fields; the report carries no price/VIN/dialog
// fields; the terminal stages are unknown_pending ALWAYS).

func pilotTestTime() time.Time {
	return time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
}

func pilotFreshCaseMaterial() PilotCaseMaterial {
	return PilotCaseMaterial{
		CaseFound: true,
		CaseID:    id.UUID("pilot-case-1"),
		CaseNo:    7,
		CreatedAt: pilotTestTime(),
		InboundEvents: []PilotInboundEvent{
			{Provider: "jivo", Channel: "chat", ProviderEventID: "evt-1", ReceivedAt: pilotTestTime()},
		},
	}
}

func pilotFullJourneyMaterial() PilotCaseMaterial {
	material := pilotFreshCaseMaterial()
	material.Vehicle = PilotVehicleMaterial{
		SchemaDefined:           true,
		IdentificationSatisfied: true,
		ConfirmedKeys:           []string{"make", "model", "year"},
		LastConfirmedAt:         pilotTestTime().Add(time.Hour),
	}
	material.Fitment = PilotFitmentMaterial{
		Positions: []PilotFitmentPositionMaterial{
			{PositionKey: "front-left", Status: "proven", ProvenAt: pilotTestTime().Add(2 * time.Hour)},
		},
	}
	material.WorkItemsCount = 1
	material.PartRequestConfirmed = true
	material.HandoffRequests = 1
	material.Corrections = 2
	material.RecoveryCases = 1
	material.Orders = []PilotOrderMaterial{
		{
			Operation: OrderStatusOperationMaterial{
				Found:   true,
				Receipt: &OrderStatusReceiptPin{ReceiptVersionNo: 1, ExternalOrderID: "ABCP-77", CreatedAt: pilotTestTime().Add(3 * time.Hour)},
			},
			Verifications: []OrderStatusVerificationMaterial{
				{Events: []OrderVerificationEvent{
					{Kind: OrderVerificationEventApproved, VerificationNo: 1, EventNo: 2, CreatedAt: pilotTestTime().Add(4 * time.Hour)},
				}},
			},
			Facts: []OrderSupplierFact{
				{FactNo: 1, Kind: OrderSupplierFactSent, SupplierOrderID: "SUP-9", HappenedAt: pilotTestTime().Add(5 * time.Hour)},
			},
		},
	}
	return material
}

func pilotStageByState(t *testing.T, outcome PilotCaseOutcome, stage string) PilotStageEvidence {
	t.Helper()
	for _, evidence := range outcome.Stages {
		if evidence.Stage == stage {
			return evidence
		}
	}
	t.Fatalf("stage %s not found", stage)
	return PilotStageEvidence{}
}

func TestPilotOutcomeStagesDictionary(t *testing.T) {
	if len(PilotOutcomeStages) != 10 {
		t.Fatalf("stage dictionary: want 10 stages, got %d", len(PilotOutcomeStages))
	}
	want := []string{
		"inquiry_accepted", "vehicle_profile_confirmed", "applicability_proven",
		"client_order_created", "manager_verified", "supplier_sent",
		"fulfilled", "paid", "cancelled", "returned",
	}
	for i, stage := range want {
		if PilotOutcomeStages[i] != stage {
			t.Fatalf("stage %d: want %s, got %s", i, stage, PilotOutcomeStages[i])
		}
	}
	if len(PilotTerminalStages) != 4 {
		t.Fatalf("terminal dictionary: want 4, got %d", len(PilotTerminalStages))
	}
	if len(PilotMeasurableStages) != 6 {
		t.Fatalf("measurable dictionary: want 6, got %d", len(PilotMeasurableStages))
	}
	for _, state := range []string{PilotStageStateProven, PilotStageStateNotReached, PilotStageStateUnknownPending} {
		if PilotStageStateWords[state] == "" {
			t.Fatalf("state word missing for %s", state)
		}
	}
}

func TestDeriveCaseOutcomeFreshCase(t *testing.T) {
	outcome := DeriveCaseOutcome(pilotFreshCaseMaterial())
	if !outcome.CaseFound {
		t.Fatal("fresh case: outcome must be found")
	}
	inquiry := pilotStageByState(t, outcome, PilotStageInquiryAccepted)
	if inquiry.State != PilotStageStateProven {
		t.Fatalf("inquiry: want proven, got %s", inquiry.State)
	}
	if inquiry.SourceWord != PilotSourceChannelEventWord {
		t.Fatalf("inquiry source: %s", inquiry.SourceWord)
	}
	for _, stage := range PilotMeasurableStages[1:] {
		evidence := pilotStageByState(t, outcome, stage)
		if evidence.State != PilotStageStateNotReached {
			t.Fatalf("%s: want not_reached, got %s", stage, evidence.State)
		}
	}
	for _, stage := range PilotTerminalStages {
		evidence := pilotStageByState(t, outcome, stage)
		if evidence.State != PilotStageStateUnknownPending {
			t.Fatalf("%s: want unknown_pending, got %s", stage, evidence.State)
		}
		if evidence.PinWord == "" {
			t.Fatalf("%s: pin word missing", stage)
		}
	}
	if outcome.Attribution.Provider != "jivo" || outcome.Attribution.Channel != "chat" || outcome.Attribution.ProviderEventID != "evt-1" {
		t.Fatalf("attribution: %+v", outcome.Attribution)
	}
	if !PilotStageEvidenceSortedByDictionary(outcome.Stages) {
		t.Fatal("stages must arrive in the dictionary order")
	}
}

func TestDeriveCaseOutcomeFullJourney(t *testing.T) {
	outcome := DeriveCaseOutcome(pilotFullJourneyMaterial())
	for _, stage := range PilotMeasurableStages {
		evidence := pilotStageByState(t, outcome, stage)
		if evidence.State != PilotStageStateProven {
			t.Fatalf("%s: want proven, got %s", stage, evidence.State)
		}
		if evidence.SourceWord == "" {
			t.Fatalf("%s: source word missing", stage)
		}
	}
	for _, stage := range PilotTerminalStages {
		evidence := pilotStageByState(t, outcome, stage)
		if evidence.State != PilotStageStateUnknownPending {
			t.Fatalf("%s terminal must stay unknown_pending on the FULL journey: got %s", stage, evidence.State)
		}
	}
	if outcome.HandoffCount != 1 || outcome.CorrectionsCount != 2 || outcome.RecoveryCount != 1 {
		t.Fatalf("counters: %+v", outcome)
	}
	if len(outcome.ChainWords) != 9 {
		t.Fatalf("chain: want 9 links, got %d", len(outcome.ChainWords))
	}
	created := pilotStageByState(t, outcome, PilotStageClientOrderCreated)
	if !contains(created.Evidence, "ABCP-77") {
		t.Fatalf("created evidence must carry the external ID: %s", created.Evidence)
	}
	sent := pilotStageByState(t, outcome, PilotStageSupplierSent)
	if !contains(sent.Evidence, "SUP-9") {
		t.Fatalf("sent evidence must carry the supplier order ID: %s", sent.Evidence)
	}
}

func TestDeriveCaseOutcomeLateFactFlip(t *testing.T) {
	without := pilotFullJourneyMaterial()
	without.Orders[0].Facts = nil
	outcomeBefore := DeriveCaseOutcome(without)
	before := pilotStageByState(t, outcomeBefore, PilotStageSupplierSent)
	if before.State != PilotStageStateNotReached {
		t.Fatalf("before the late fact: want not_reached, got %s", before.State)
	}
	with := pilotFullJourneyMaterial()
	outcomeAfter := DeriveCaseOutcome(with)
	after := pilotStageByState(t, outcomeAfter, PilotStageSupplierSent)
	if after.State != PilotStageStateProven {
		t.Fatalf("after the late fact: want proven, got %s", after.State)
	}
}

func TestDeriveCaseOutcomeAttributionUnknown(t *testing.T) {
	material := pilotFreshCaseMaterial()
	material.InboundEvents = nil
	outcome := DeriveCaseOutcome(material)
	if outcome.Attribution.Found {
		t.Fatal("attribution without events must not be found")
	}
	if outcome.Attribution.CampaignWord != PilotAttributionUnknownWord {
		t.Fatalf("campaign word: %s", outcome.Attribution.CampaignWord)
	}
	if outcome.Attribution.ReferrerWord != PilotAttributionUnknownWord {
		t.Fatalf("referrer word: %s", outcome.Attribution.ReferrerWord)
	}
	if !contains(outcome.Attribution.Word, PilotAttributionMissingWord) {
		t.Fatalf("attribution word: %s", outcome.Attribution.Word)
	}
}

func TestDerivePilotReportAggregate(t *testing.T) {
	full := DeriveCaseOutcome(pilotFullJourneyMaterial())
	fresh := DeriveCaseOutcome(pilotFreshCaseMaterial())
	report := DerivePilotReport([]PilotCaseOutcome{full, fresh}, "2026-10")
	if report.CaseVolume != 2 || report.SampleSize != 2 {
		t.Fatalf("volume: %+v", report)
	}
	if report.PeriodWord != "2026-10" {
		t.Fatalf("period: %s", report.PeriodWord)
	}
	if report.StageCounts[PilotStageInquiryAccepted] != 2 {
		t.Fatalf("inquiry count: %d", report.StageCounts[PilotStageInquiryAccepted])
	}
	if report.StageCounts[PilotStageSupplierSent] != 1 {
		t.Fatalf("sent count: %d", report.StageCounts[PilotStageSupplierSent])
	}
	if report.StageCounts[PilotStageFulfilled] != 0 {
		t.Fatalf("terminal proven count must be 0: %d", report.StageCounts[PilotStageFulfilled])
	}
	if report.CorrectionsCount != 2 || report.HandoffCount != 1 || report.RecoveryCount != 1 {
		t.Fatalf("counters: %+v", report)
	}
	if len(report.Conversions) != 5 {
		t.Fatalf("conversions: want 5, got %d", len(report.Conversions))
	}
	first := report.Conversions[0]
	if first.From != PilotStageInquiryAccepted || first.To != PilotStageVehicleProfileConfirmed {
		t.Fatalf("first conversion: %+v", first)
	}
	if first.Numerator != 1 || first.Denominator != 2 {
		t.Fatalf("first conversion numbers: %+v", first)
	}
	if report.UnknownShare.Numerator != 2 || report.UnknownShare.Denominator != 2 {
		t.Fatalf("unknown share: %+v", report.UnknownShare)
	}
	if !contains(report.BoundaryWord, "baseline") {
		t.Fatalf("boundary word: %s", report.BoundaryWord)
	}
}

func TestDerivePilotReportEmptySample(t *testing.T) {
	report := DerivePilotReport(nil, "2026-10")
	if report.CaseVolume != 0 {
		t.Fatalf("volume: %d", report.CaseVolume)
	}
	if report.EmptySampleWord != PilotEmptySampleWord {
		t.Fatalf("empty sample word: %s", report.EmptySampleWord)
	}
	if len(report.Conversions) != 0 {
		t.Fatalf("conversions on the empty sample: %d", len(report.Conversions))
	}
	if report.UnknownShare.Denominator != 0 {
		t.Fatalf("share denominator on the empty sample: %d", report.UnknownShare.Denominator)
	}
}

func TestPilotMeasurementDictionary(t *testing.T) {
	seen := map[string]bool{}
	for _, indicator := range PilotMeasurementDictionary {
		if indicator.ID == "" || indicator.Formula == "" || indicator.Grain == "" || indicator.OwningSource == "" {
			t.Fatalf("incomplete indicator: %+v", indicator)
		}
		if indicator.Version != PilotDictionaryVersion {
			t.Fatalf("version: %s", indicator.Version)
		}
		if indicator.Owner != PilotDictionaryOwnerWord {
			t.Fatalf("owner: %s", indicator.Owner)
		}
		if seen[indicator.ID] {
			t.Fatalf("duplicate indicator id: %s", indicator.ID)
		}
		seen[indicator.ID] = true
	}
	for _, want := range []string{"case_volume", "corrections_count", "handoff_count", "recovery_count", "unknown_outcome_share"} {
		if !seen[want] {
			t.Fatalf("missing indicator: %s", want)
		}
	}
	for i := 0; i+1 < len(PilotMeasurableStages); i++ {
		want := "conversion_" + PilotMeasurableStages[i] + "_" + PilotMeasurableStages[i+1]
		if !seen[want] {
			t.Fatalf("missing conversion indicator: %s", want)
		}
	}
	if len(PilotNotMeasurableIndicators) != 4 {
		t.Fatalf("non-measurable: want 4, got %d", len(PilotNotMeasurableIndicators))
	}
	for _, want := range []string{"revenue", "margin", "variable_costs", "human_minutes"} {
		found := false
		for _, indicator := range PilotNotMeasurableIndicators {
			if indicator.ID == want {
				found = true
				if indicator.Reason == "" {
					t.Fatalf("non-measurable %s must carry the reason word", want)
				}
			}
		}
		if !found {
			t.Fatalf("missing non-measurable indicator: %s", want)
		}
	}
}

func TestPilotStructuralHonestyProbes(t *testing.T) {
	nonMeasurable := reflect.TypeOf(PilotNotMeasurableIndicator{})
	for i := 0; i < nonMeasurable.NumField(); i++ {
		if nonMeasurable.Field(i).Type.Kind() != reflect.String {
			t.Fatalf("PilotNotMeasurableIndicator must carry STRING fields only (no numbers, never a zero): field %s is %s",
				nonMeasurable.Field(i).Name, nonMeasurable.Field(i).Type.Kind())
		}
	}
	forbidden := []string{"price", "revenue", "amount", "cost", "vin", "phone", "dialog", "margin"}
	probeStructFields(t, reflect.TypeOf(PilotReport{}), forbidden, "PilotReport")
	probeStructFields(t, reflect.TypeOf(PilotCaseOutcome{}), forbidden, "PilotCaseOutcome")
	probeStructFields(t, reflect.TypeOf(PilotStageEvidence{}), forbidden, "PilotStageEvidence")
	probeStructFields(t, reflect.TypeOf(PilotAttributionView{}), forbidden, "PilotAttributionView")
	outcome := DeriveCaseOutcome(pilotFullJourneyMaterial())
	for _, stage := range PilotTerminalStages {
		evidence := pilotStageByState(t, outcome, stage)
		if evidence.State != PilotStageStateUnknownPending {
			t.Fatalf("terminal stage %s must be unknown_pending ALWAYS: %s", stage, evidence.State)
		}
	}
}

func probeStructFields(t *testing.T, typ reflect.Type, forbidden []string, name string) {
	t.Helper()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		lower := field.Name
		for _, word := range forbidden {
			if contains(lower, word) {
				t.Fatalf("%s must not carry the field %s (the honest boundary of the aggregate)", name, field.Name)
			}
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
