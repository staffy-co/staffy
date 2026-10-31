package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.14 unit matrix (the domain half): the five dictionaries of
// the slice (the summary kind, the staleness words, the claim ref types,
// the context fact statuses — the render mirror of the work.go
// derivations — and the context commitment statuses), the record guard
// (the material bounds, the ref discipline and the byte-precision
// anti-hallucination probe), the staleness derivation (every dimension
// of the watermark comparison with the honest counters), the read-time
// block assembly (the twelve blocks over the existing derivations of
// 2.2..2.13, the empty control branch and the read-time control branch)
// and the outbox perimeter (the identifiers, the words and the counters
// — the text and the claims never travel).

// contextID builds a distinct identifier of the unit matrix (newID is a
// constant and otherID is a single second value — the grouping proofs
// need more than two distinct rows).
func contextID(n int) id.UUID {
	value, err := id.Parse(fmt.Sprintf("019a2f4c-1000-7000-8000-%012d", n))
	if err != nil {
		panic(err)
	}
	return value
}

// summaryTextOf is the summary text of the fixtures: three claim spans
// (two linked, one disputed) live inside it byte-for-byte.
const summaryTextOf = "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1. Уточняющий вопрос клиента ожидает ответа. Срок доставки клиенту не назван."

// recordedSummaryOf is the summary row of the fixtures: the single kind
// word, the three claims (the address span linked to a fact version, the
// question span linked to an observation, the deadline span honestly
// disputed) and the watermark of the recording moment.
func recordedSummaryOf() *RecordedSummary {
	return &RecordedSummary{
		TenantID: newID(), CaseID: newID(), SummaryNo: 1,
		Kind:    SummaryKindAI,
		ActorID: newID(),
		Text:    summaryTextOf,
		Claims: []SummaryClaim{
			{RefType: ClaimRefFactVersion, RefID: contextID(1), Text: "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1."},
			{RefType: ClaimRefObservation, RefID: contextID(2), Text: "Уточняющий вопрос клиента ожидает ответа."},
			{Disputed: true, Text: "Срок доставки клиенту не назван."},
		},
		Watermark: SummaryWatermark{EventCount: 10, FactVersionCount: 4, OwnerEpoch: 1, ModeVersion: 2},
		CreatedAt: time.Now(),
	}
}

// TestDecisionContextDictionariesClosed pins the closure of the five
// dictionaries: the single kind word (the operator summaries are the
// honest Never), the two staleness words, the two ref types, the three
// render fact statuses and the three commitment render statuses. A
// medical word in any dictionary is the AC-4 probe — the dictionaries
// are exactly the fixed lists.
func TestDecisionContextDictionariesClosed(t *testing.T) {
	if len(SummaryKinds) != 1 || SummaryKinds[0] != SummaryKindAI {
		t.Fatalf("the summary kind dictionary carries the single producer word ai_summary, got %v", SummaryKinds)
	}
	if SummaryKindInDictionary("operator_note") || !SummaryKindInDictionary(SummaryKindAI) {
		t.Fatalf("the summary kind dictionary is closed on ai_summary")
	}
	if len(ContextStalenessWords) != 2 || ContextStalenessWords[0] != ContextStalenessActual || ContextStalenessWords[1] != ContextStalenessStale {
		t.Fatalf("the staleness dictionary is exactly actual|stale, got %v", ContextStalenessWords)
	}
	if len(ClaimRefTypes) != 2 || ClaimRefTypes[0] != ClaimRefFactVersion || ClaimRefTypes[1] != ClaimRefObservation {
		t.Fatalf("the claim ref-type dictionary is exactly fact_version|observation, got %v", ClaimRefTypes)
	}
	if len(ContextFactStatuses) != 3 || ContextFactStatuses[0] != ContextFactConfirmed || ContextFactStatuses[1] != ContextFactUnconfirmed || ContextFactStatuses[2] != ContextFactConflicting {
		t.Fatalf("the context fact-status dictionary is exactly confirmed|unconfirmed|conflicting, got %v", ContextFactStatuses)
	}
	for _, word := range ContextFactStatuses {
		if ContextFactStatusText(word) == "" {
			t.Fatalf("the render text of the fact status %s is empty", word)
		}
	}
	if len(ContextCommitmentStatuses) != 3 || ContextCommitmentStatuses[2] != ContextCommitmentConfirmed {
		t.Fatalf("the context commitment-status dictionary is exactly draft|offered|confirmed, got %v", ContextCommitmentStatuses)
	}
	if AIMarkingText == "" || ContextStaleText == "" || ReceiptAbsentText == "" || RecommendationProjectionText == "" {
		t.Fatalf("the honest marking texts are never empty")
	}
	if MinSummaryTextRunes != 1 || MaxSummaryTextRunes != 1000 || MinSummaryClaims != 1 || MaxSummaryClaims != 32 || MinClaimTextRunes != 1 || MaxClaimTextRunes != 200 {
		t.Fatalf("the material limits are the fixed dictionary: text 1..1000, claims 1..32, claim text 1..200")
	}
}

// TestGuardRecordCaseSummary pins the record guard: the material bounds
// (the runes, the claims), the ref discipline (every claim linked or
// honestly disputed, the ref word of the closed dictionary WITH the id)
// and the byte-precision anti-hallucination probe (an assertion that is
// not a line of the summary is a fabrication the guard refuses).
func TestGuardRecordCaseSummary(t *testing.T) {
	validClaims := []SummaryClaim{
		{RefType: ClaimRefFactVersion, RefID: newID(), Text: "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1."},
		{RefType: ClaimRefObservation, RefID: newID(), Text: "Уточняющий вопрос клиента ожидает ответа."},
		{Disputed: true, Text: "Срок доставки клиенту не назван."},
	}

	t.Run("valid linked and disputed claims pass", func(t *testing.T) {
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, validClaims); err != nil {
			t.Fatalf("the honest summary material passes, got %v", err)
		}
	})

	t.Run("disputed claim with a ref passes the OR discipline", func(t *testing.T) {
		claims := append(append([]SummaryClaim{}, validClaims[:2]...), SummaryClaim{
			RefType: ClaimRefFactVersion, RefID: newID(), Disputed: true,
			Text: "Срок доставки клиенту не назван.",
		})
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, claims); err != nil {
			t.Fatalf("a disputed claim may carry a ref (the OR formula of the material CHECK), got %v", err)
		}
	})

	t.Run("kind outside the dictionary refuses", func(t *testing.T) {
		if err := GuardRecordCaseSummary("operator_note", summaryTextOf, validClaims); err == nil {
			t.Fatalf("the operator summary kind is the honest Never — the guard refuses")
		}
	})

	t.Run("empty text refuses", func(t *testing.T) {
		if err := GuardRecordCaseSummary(SummaryKindAI, "", validClaims); err == nil {
			t.Fatalf("an empty summary text refuses (1..1000 runes)")
		}
	})

	t.Run("text over the limit refuses", func(t *testing.T) {
		long := strings.Repeat("а", MaxSummaryTextRunes+1)
		if err := GuardRecordCaseSummary(SummaryKindAI, long, validClaims); err == nil {
			t.Fatalf("a 1001-rune summary text refuses")
		}
		boundaryClaims := []SummaryClaim{{Disputed: true, Text: "а"}}
		if err := GuardRecordCaseSummary(SummaryKindAI, strings.Repeat("а", MaxSummaryTextRunes), boundaryClaims); err != nil {
			t.Fatalf("the exact 1000-rune boundary passes, got %v", err)
		}
	})

	t.Run("claim count bounds refuse", func(t *testing.T) {
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, nil); err == nil {
			t.Fatalf("a summary without claims refuses (1..32)")
		}
		many := make([]SummaryClaim, 0, MaxSummaryClaims+1)
		for i := 0; i <= MaxSummaryClaims; i++ {
			many = append(many, SummaryClaim{Disputed: true, Text: "Срок доставки клиенту не назван."})
		}
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, many); err == nil {
			t.Fatalf("33 claims refuse")
		}
		many = many[:MaxSummaryClaims]
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, many); err != nil {
			t.Fatalf("the exact 32-claim boundary passes, got %v", err)
		}
	})

	t.Run("claim text over the limit refuses", func(t *testing.T) {
		long := strings.Repeat("а", MaxClaimTextRunes+1)
		claims := []SummaryClaim{{Disputed: true, Text: long}}
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf+long, claims); err == nil {
			t.Fatalf("a 201-rune claim text refuses")
		}
	})

	t.Run("unlinked undisputed claim refuses", func(t *testing.T) {
		claims := append(append([]SummaryClaim{}, validClaims[:2]...), SummaryClaim{Text: "Срок доставки клиенту не назван."})
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, claims); err == nil {
			t.Fatalf("a claim without a ref and without the disputed mark refuses (AD-7: sourced or honestly disputed)")
		}
	})

	t.Run("ref word without id refuses", func(t *testing.T) {
		claims := append(append([]SummaryClaim{}, validClaims[:2]...), SummaryClaim{
			RefType: ClaimRefFactVersion, Text: "Срок доставки клиенту не назван.",
		})
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, claims); err == nil {
			t.Fatalf("a claim carrying the ref word without the id refuses")
		}
	})

	t.Run("id without the dictionary word refuses", func(t *testing.T) {
		claims := append(append([]SummaryClaim{}, validClaims[:2]...), SummaryClaim{
			RefID: newID(), Disputed: true, Text: "Срок доставки клиенту не назван.",
		})
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, claims); err == nil {
			t.Fatalf("a claim carrying the id without the ref-type word refuses")
		}
	})

	t.Run("ref type outside the dictionary refuses", func(t *testing.T) {
		claims := append(append([]SummaryClaim{}, validClaims[:2]...), SummaryClaim{
			RefType: "mandate", RefID: newID(), Text: "Срок доставки клиенту не назван.",
		})
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, claims); err == nil {
			t.Fatalf("a foreign ref-type word refuses (the closed dictionary)")
		}
	})

	t.Run("claim text absent from the summary refuses", func(t *testing.T) {
		claims := append(append([]SummaryClaim{}, validClaims[:2]...), SummaryClaim{
			Disputed: true, Text: "Клиент отказался от доставки.",
		})
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, claims); err == nil {
			t.Fatalf("a fabricated assertion refuses (the anti-hallucination probe)")
		}
	})

	t.Run("byte precision is not a paraphrase", func(t *testing.T) {
		claims := []SummaryClaim{{RefType: ClaimRefFactVersion, RefID: newID(), Text: "КЛИЕНТ НАЗВАЛ АДРЕС ДОСТАВКИ: КАЗАНЬ, УЛИЦА КРЕМЛЁВСКАЯ, 1."}}
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, claims); err == nil {
			t.Fatalf("the claim must be present byte-for-byte — the case difference is a fabrication")
		}
		span := []SummaryClaim{{RefType: ClaimRefFactVersion, RefID: newID(), Text: "Казань, улица Кремлёвская, 1"}}
		if err := GuardRecordCaseSummary(SummaryKindAI, summaryTextOf, span); err != nil {
			t.Fatalf("an inner span passes (a byte substring of the summary), got %v", err)
		}
	})
}

// TestDeriveContextStaleness pins the staleness derivation: every
// dimension of the watermark comparison decides the word, the honest
// counters speak of the moved dimensions only, the negative counters
// clamp to zero and the summary row never mutates (the derivation is
// read-time — a stale summary renders with the sources beside it, never
// as the actual one).
func TestDeriveContextStaleness(t *testing.T) {
	base := SummaryWatermark{EventCount: 10, FactVersionCount: 4, OwnerEpoch: 1, ModeVersion: 2}

	t.Run("no summary is the zero view", func(t *testing.T) {
		view := DeriveContextStaleness(nil, base)
		if view.Word != "" || view.NewEvents != 0 || view.OwnerChanged {
			t.Fatalf("the absent summary derives the zero view, got %+v", view)
		}
	})

	t.Run("equal watermark is actual", func(t *testing.T) {
		summary := recordedSummaryOf()
		view := DeriveContextStaleness(summary, base)
		if view.Word != ContextStalenessActual || view.StalenessCountersText() != "" {
			t.Fatalf("the equal material derives actual with no counters, got %q %+v", view.Word, view)
		}
	})

	t.Run("new events decide stale with the counter", func(t *testing.T) {
		summary := recordedSummaryOf()
		current := base
		current.EventCount = 13
		view := DeriveContextStaleness(summary, current)
		if view.Word != ContextStalenessStale || view.NewEvents != 3 || view.NewFactVersions != 0 {
			t.Fatalf("three new events decide stale with the counter, got %+v", view)
		}
		if !strings.Contains(view.StalenessCountersText(), "новых событий: 3") {
			t.Fatalf("the counters text names the moved dimension, got %q", view.StalenessCountersText())
		}
	})

	t.Run("new fact versions decide stale with the counter", func(t *testing.T) {
		summary := recordedSummaryOf()
		current := base
		current.FactVersionCount = 6
		view := DeriveContextStaleness(summary, current)
		if view.Word != ContextStalenessStale || view.NewFactVersions != 2 {
			t.Fatalf("two new fact versions decide stale, got %+v", view)
		}
		if !strings.Contains(view.StalenessCountersText(), "новых версий фактов: 2") {
			t.Fatalf("the counters text names the fact dimension, got %q", view.StalenessCountersText())
		}
	})

	t.Run("owner change decides stale", func(t *testing.T) {
		summary := recordedSummaryOf()
		current := base
		current.OwnerEpoch = 2
		view := DeriveContextStaleness(summary, current)
		if !view.OwnerChanged || view.Word != ContextStalenessStale {
			t.Fatalf("the owner change decides stale, got %+v", view)
		}
		if !strings.Contains(view.StalenessCountersText(), "владелец сменился") {
			t.Fatalf("the counters text names the owner dimension, got %q", view.StalenessCountersText())
		}
	})

	t.Run("mode change decides stale", func(t *testing.T) {
		summary := recordedSummaryOf()
		current := base
		current.ModeVersion = 3
		view := DeriveContextStaleness(summary, current)
		if !view.ModeChanged || view.Word != ContextStalenessStale {
			t.Fatalf("the mode change decides stale, got %+v", view)
		}
		if !strings.Contains(view.StalenessCountersText(), "режим сменился") {
			t.Fatalf("the counters text names the mode dimension, got %q", view.StalenessCountersText())
		}
	})

	t.Run("every moved dimension speaks once joined", func(t *testing.T) {
		summary := recordedSummaryOf()
		current := SummaryWatermark{EventCount: 12, FactVersionCount: 5, OwnerEpoch: 3, ModeVersion: 7}
		view := DeriveContextStaleness(summary, current)
		text := view.StalenessCountersText()
		for _, part := range []string{"новых событий: 2", "новых версий фактов: 1", "владелец сменился", "режим сменился"} {
			if !strings.Contains(text, part) {
				t.Fatalf("the counters text carries %q, got %q", part, text)
			}
		}
		if strings.Count(text, ";") != 3 {
			t.Fatalf("four parts join by three semicolons, got %q", text)
		}
	})

	t.Run("negative counters clamp to actual", func(t *testing.T) {
		summary := recordedSummaryOf()
		current := SummaryWatermark{EventCount: 5, FactVersionCount: 2, OwnerEpoch: 1, ModeVersion: 2}
		view := DeriveContextStaleness(summary, current)
		if view.Word != ContextStalenessActual || view.NewEvents != 0 || view.NewFactVersions != 0 {
			t.Fatalf("the recycled material clamps to actual (the defensive backstop), got %+v", view)
		}
	})

	t.Run("the derivation never mutates the summary", func(t *testing.T) {
		summary := recordedSummaryOf()
		before := summary.Watermark
		_ = DeriveContextStaleness(summary, SummaryWatermark{EventCount: 99, FactVersionCount: 99, OwnerEpoch: 9, ModeVersion: 9})
		if summary.Watermark != before {
			t.Fatalf("the staleness derivation is read-time — the summary row never mutates")
		}
	})
}

// The fixture builders of the assembly matrix (the raw material the
// store reads in ONE transaction — every block derives read-time).

func contextModeTerm(version int64, mode string) ModeTerm {
	return ModeTerm{
		TenantID: newID(), CaseID: newID(), ConversationID: newID(), TermID: newID(),
		ModeVersion: version, Mode: mode, Scope: ModeScopeConversation,
		Origin: ModeOriginIngestInitial, ActorID: newID(), CreatedAt: time.Now().Add(-time.Hour),
	}
}

func contextOwnershipTerm(epoch int64, kind string) OwnershipTerm {
	return OwnershipTerm{
		TenantID: newID(), CaseID: newID(), TermID: newID(), Epoch: epoch,
		OwnerKind: kind, OwnerRef: newID(), WritableSurface: WritableSurfaceInbox,
		Origin: ModeOriginIngestInitial, ActorID: newID(), CreatedAt: time.Now(),
	}
}

func contextVerification(kind, level string, expires time.Time) VerificationRecord {
	return VerificationRecord{
		TenantID: newID(), CaseID: newID(), ConversationID: newID(), VerificationID: newID(),
		Kind: kind, Method: MethodKnownNumberCallback, Outcome: VerificationPassed,
		AssuranceLevel: level, SubjectHandle: "+7-900-000-00-01",
		IssuedAt: time.Now().Add(-time.Hour), ExpiresAt: expires, CreatedAt: time.Now().Add(-time.Hour),
	}
}

func contextFactVersion(factID id.UUID, versionNo int64, materialKey, digest string) FactVersion {
	return FactVersion{
		TenantID: newID(), CaseID: newID(), FactID: factID, WorkItemID: contextID(90),
		VersionNo: versionNo, MaterialKey: materialKey, Value: []byte(`"` + digest + `"`), ValueDigest: digest,
		SourceType: "event", SourceID: "evt-1", Channel: "web_chat",
		ActorID: newID(), ObservedAt: time.Now().Add(-time.Minute), ReceivedAt: time.Now(), CreatedAt: time.Now(),
	}
}

func contextFactConfirmation(factID id.UUID, versionNo int64) FactConfirmation {
	return FactConfirmation{
		TenantID: newID(), CaseID: newID(), FactID: factID, ConfirmationID: newID(),
		VersionNo: versionNo, ConfirmerType: "operator", ActorID: newID(), CreatedAt: time.Now(),
	}
}

func contextObservation(no int64, materialKey, status string, observedAt time.Time) Observation {
	owningSystem, projection := OwningSystemCatalog, ProjectionPriceQuote
	switch materialKey {
	case MaterialFieldAvailability:
		projection = ProjectionStockCount
	case MaterialFieldOrderStatus:
		owningSystem, projection = OwningSystemErp, ProjectionStatusWord
	}
	return Observation{
		TenantID: newID(), CaseID: newID(), WorkItemID: contextID(90), ObservationID: newID(),
		ObservationNo: no, MaterialKey: materialKey, OwningSystem: owningSystem, Projection: projection,
		Status: status, ObservedAt: observedAt, ReceivedAt: time.Now(),
		ExternalID: "erp-123", ExternalVersion: "v7", ActorID: newID(), CorrelationID: newID(),
		CreatedAt: observedAt,
	}
}

func contextCommitmentVersion(commitmentID id.UUID, versionNo int64, status string, unknowns []string) CommitmentVersion {
	return CommitmentVersion{
		TenantID: newID(), CaseID: newID(), WorkItemID: contextID(90), CommitmentID: commitmentID,
		VersionNo: versionNo, Status: status, ActionSet: []string{CommitmentActionPrepareOrder},
		ValidUntil: time.Now().Add(24 * time.Hour), MaterialUnknowns: unknowns,
		OwnerEpoch: 2, LineageWord: "release-line-1", ActorID: newID(), CreatedAt: time.Now(),
	}
}

func contextConfirmation(commitmentID id.UUID, versionNo int64, expires time.Time) ConfirmationRecord {
	return ConfirmationRecord{
		TenantID: newID(), CaseID: newID(), WorkItemID: contextID(90), CommitmentID: commitmentID,
		VersionNo: versionNo, ConfirmationID: newID(), Method: CommitmentMethodExplicitText,
		Channel: "web_chat", ExpiresAt: expires, ConfirmedAt: time.Now().Add(-time.Minute), ActorID: newID(),
	}
}

func contextOutcome(outcomeNo int64, createdAt time.Time) PolicyOutcome {
	return PolicyOutcome{
		TenantID: newID(), CaseID: newID(), ConversationID: newID(), WorkItemID: contextID(90),
		ProposalID: newID(), OutcomeID: newID(), OutcomeNo: outcomeNo,
		OutcomeClass: OutcomeClarify, Reason: ReasonMandatoryMissing, RuleVersion: "rule-v1",
		Intent:                 IntentServiceRequest,
		MandatoryFacts:         []MandatoryFactStatus{{MaterialKey: "service_kind", Source: MissingReasonMandatory, State: MandatoryStateAsk}},
		NextSteps:              []NextStep{{Step: StepAskFact, MaterialKey: "service_kind"}, {Step: StepHumanHandoff}},
		ActiveAlternatives:     []string{AlternativeTextDescription},
		DegradedPermittedPaths: []string{AlternativeCallback},
		ActorID:                newID(), CreatedAt: createdAt,
	}
}

// TestDeriveDecisionContextBlocks pins the block assembly: the twelve
// blocks derive from the raw material through the EXISTING derivations
// of 2.2..2.13 (the lifecycle state, the ladder words, the assurance,
// the ownership epoch, the fact groups, the observation freshness, the
// delivery state, the commitment validity, the preferences revision
// truth, the plan words) and every rendered word belongs to its fixed
// dictionary.
func TestDeriveDecisionContextBlocks(t *testing.T) {
	now := time.Now()
	request := handoffRequestOf()
	assigned := handoffEventOf(request, 1, HandoffEventAssigned)
	input := DecisionContextInput{
		Now:            now,
		HandoffRequest: &request, HandoffEvents: []HandoffEvent{assigned},
		WorkItems: []WorkItemCurrent{
			{WorkItemID: contextID(41), State: WorkItemStateUntriaged, Title: "Замена детали", Type: "service", Kind: "work_item"},
			{WorkItemID: contextID(42), State: WorkItemStateResolved, Title: "Закрытый вопрос", Type: "service", Kind: "work_item"},
		},
		ModeTerms: []ModeTerm{contextModeTerm(1, ModeSales), contextModeTerm(2, ModeHumanRequested)},
		Verifications: []VerificationRecord{
			contextVerification(VerificationKindIdentity, AssuranceVerified, now.Add(time.Hour)),
		},
		OwnershipTerms: []OwnershipTerm{contextOwnershipTerm(1, OwnerKindHuman), contextOwnershipTerm(2, OwnerKindStaffy)},
		FactVersions: []FactVersion{
			contextFactVersion(contextID(11), 1, "address", "digest-a"),
			contextFactVersion(contextID(12), 1, "service_kind", "digest-x"),
			contextFactVersion(contextID(12), 2, "service_kind", "digest-y"),
			contextFactVersion(contextID(13), 1, "urgency", "digest-u"),
		},
		FactConfirmations: []FactConfirmation{
			contextFactConfirmation(contextID(11), 1),
		},
		Observations: []Observation{
			contextObservation(1, MaterialFieldPrice, ObservationStatusOk, now.Add(-10*time.Second)),
			contextObservation(1, MaterialFieldAvailability, ObservationStatusOk, now.Add(2*time.Minute)),
			contextObservation(2, MaterialFieldAvailability, ObservationStatusOk, now.Add(-2*time.Minute)),
			contextObservation(1, MaterialFieldOrderStatus, ObservationStatusFailed, now.Add(-time.Minute)),
		},
		Replies: []ReplyDeliveryMaterial{
			{ReplyID: contextID(31), Dispatched: false},
			{ReplyID: contextID(32), Dispatched: true, Events: []ReplyDeliveryEvent{{
				TenantID: newID(), CaseID: newID(), ReplyID: contextID(32), EventID: newID(),
				EventNo: 1, Kind: DeliveryEvidenceDeliveredConfirmed, ActorID: newID(), CreatedAt: now,
			}}},
		},
		CommitmentVersions: []CommitmentVersion{
			contextCommitmentVersion(contextID(21), 1, CommitmentStatusOffered, nil),
			contextCommitmentVersion(contextID(22), 1, CommitmentStatusDraft, []string{CommitmentTermScope}),
		},
		Confirmations: []ConfirmationRecord{
			contextConfirmation(contextID(21), 1, now.Add(time.Hour)),
		},
		CurrentOwnerEpoch: 2, CurrentLineage: "release-line-1",
		Accommodations: []AccommodationRecord{
			{TenantID: newID(), CaseID: newID(), RecordID: newID(), Key: AccommodationSlowedPace, Revision: 1, ActorID: newID(), RecordedAt: now},
			{TenantID: newID(), CaseID: newID(), RecordID: newID(), Key: AccommodationTextFirst, Revision: 1, ActorID: newID(), RecordedAt: now},
			{TenantID: newID(), CaseID: newID(), RecordID: newID(), Key: AccommodationTextFirst, Revision: 2, Cleared: true, ActorID: newID(), RecordedAt: now},
		},
		Outcomes: []PolicyOutcome{
			contextOutcome(1, now.Add(-2*time.Hour)),
			contextOutcome(2, now.Add(-time.Hour)),
		},
		Summary:   recordedSummaryOf(),
		Watermark: SummaryWatermark{EventCount: 12, FactVersionCount: 4, OwnerEpoch: 2, ModeVersion: 2},
	}
	view := DeriveDecisionContext(input)

	if !view.Handoff.Present || view.Handoff.Reason != HandoffReasonClientRequest || view.Handoff.State != HandoffStateAssigned {
		t.Fatalf("the open handoff block derives the reason word and the assigned state, got %+v", view.Handoff)
	}
	if !view.Handoff.DeadlineAt.After(now) || view.Handoff.AckDeadline != request.AckDeadline {
		t.Fatalf("the handoff block derives the live round deadline, got %+v", view.Handoff)
	}
	if len(view.WorkItems) != 1 || view.WorkItems[0].WorkItemID != contextID(41) || view.WorkItems[0].State != WorkItemStateUntriaged {
		t.Fatalf("the terminal work items never render into the active context, got %+v", view.WorkItems)
	}
	if !view.Mode.Present || view.Mode.Mode != ModeHumanRequested || view.Mode.ModeVersion != 2 {
		t.Fatalf("the mode block derives the active conversation term (the maximal version), got %+v", view.Mode)
	}
	if view.Assurance.IdentityLevel != AssuranceVerified || view.Assurance.IdentityMethod != MethodKnownNumberCallback {
		t.Fatalf("the assurance block derives the level and the method of the latest record, got %+v", view.Assurance)
	}
	if !view.Owner.Present || view.Owner.OwnerKind != OwnerKindStaffy || view.Owner.Epoch != 2 {
		t.Fatalf("the owner block derives the term of the maximal epoch, got %+v", view.Owner)
	}
	if len(view.Facts.Confirmed) != 1 || view.Facts.Confirmed[0].MaterialKey != "address" || view.Facts.Confirmed[0].Status != ContextFactConfirmed {
		t.Fatalf("the confirmed fact group carries the address fact with the render word, got %+v", view.Facts.Confirmed)
	}
	if len(view.Facts.Conflicting) != 1 || view.Facts.Conflicting[0].MaterialKey != "service_kind" || view.Facts.Conflicting[0].Status != ContextFactConflicting {
		t.Fatalf("the conflicting fact group carries the two-digest fact, got %+v", view.Facts.Conflicting)
	}
	if len(view.Facts.Unconfirmed) != 1 || view.Facts.Unconfirmed[0].MaterialKey != "urgency" || view.Facts.Unconfirmed[0].Status != ContextFactUnconfirmed {
		t.Fatalf("the unverified derivation renders as the unconfirmed word (the mirror), got %+v", view.Facts.Unconfirmed)
	}
	if row := view.Facts.Unconfirmed[0]; row.SourceType != "event" || row.SourceID != "evt-1" || row.Channel != "web_chat" || row.VersionNo != 1 {
		t.Fatalf("the fact rows carry the provenance and the version, got %+v", row)
	}
	if len(view.Observations) != 3 {
		t.Fatalf("the observations group by the work item and the material field (three fields), got %d rows", len(view.Observations))
	}
	byField := map[string]ObservationContextRow{}
	for _, row := range view.Observations {
		byField[row.MaterialKey] = row
	}
	if byField[MaterialFieldPrice].FreshnessWord != ObservationStateFresh {
		t.Fatalf("the price observation inside the 300s window derives fresh, got %+v", byField[MaterialFieldPrice])
	}
	if byField[MaterialFieldAvailability].FreshnessWord != ObservationStateStale {
		t.Fatalf("the availability observation past the 60s window derives stale (the maximal row wins), got %+v", byField[MaterialFieldAvailability])
	}
	if byField[MaterialFieldOrderStatus].FreshnessWord != ObservationStateFailed || byField[MaterialFieldOrderStatus].EvidenceStatus != ObservationStatusFailed {
		t.Fatalf("the failed read never dresses up as fresh, got %+v", byField[MaterialFieldOrderStatus])
	}
	if byField[MaterialFieldPrice].ConflictRule != ConflictRuleShowConflict || byField[MaterialFieldOrderStatus].OwningSystem != OwningSystemErp {
		t.Fatalf("the observation rows carry the matrix words, got %+v", byField[MaterialFieldPrice])
	}
	if len(view.Deliveries) != 2 {
		t.Fatalf("both reply materials derive their delivery lines, got %d", len(view.Deliveries))
	}
	deliveryByReply := map[id.UUID]DeliveryContextRow{}
	for _, row := range view.Deliveries {
		deliveryByReply[row.ReplyID] = row
	}
	if deliveryByReply[contextID(31)].State != ReplyDeliveryPrepared {
		t.Fatalf("the undispatched reply derives prepared, got %+v", deliveryByReply[contextID(31)])
	}
	if deliveryByReply[contextID(32)].State != ReplyDeliveryConfirmed || deliveryByReply[contextID(32)].LastKind != DeliveryEvidenceDeliveredConfirmed {
		t.Fatalf("the dispatched reply with the receipt derives confirmed with the evidence word, got %+v", deliveryByReply[contextID(32)])
	}
	if len(view.Commitments) != 2 {
		t.Fatalf("both commitments derive their rows, got %d", len(view.Commitments))
	}
	commitmentByID := map[id.UUID]CommitmentContextRow{}
	for _, row := range view.Commitments {
		commitmentByID[row.CommitmentID] = row
	}
	if commitmentByID[contextID(21)].Status != ContextCommitmentConfirmed || commitmentByID[contextID(21)].ConfirmationStatus != ConfirmationValidityValid {
		t.Fatalf("the offered version with the valid confirmation renders the confirmed word, got %+v", commitmentByID[contextID(21)])
	}
	if commitmentByID[contextID(22)].Status != ContextCommitmentDraft || commitmentByID[contextID(22)].ConfirmationStatus != "" {
		t.Fatalf("the draft version renders the draft word without a confirmation, got %+v", commitmentByID[contextID(22)])
	}
	if row := commitmentByID[contextID(22)]; !inDictionary(row.TermsUnknown, CommitmentTermScope) || inDictionary(row.TermsKnown, CommitmentTermScope) || row.ReceiptText != ReceiptAbsentText {
		t.Fatalf("the unknown term lands in the honest hole and the receipt line stays absent, got %+v", row)
	}
	if len(view.Preferences.AccommodationKeys) != 1 || view.Preferences.AccommodationKeys[0] != AccommodationSlowedPace {
		t.Fatalf("the cleared preference leaves the active set (the revision truth), got %v", view.Preferences.AccommodationKeys)
	}
	if len(view.Preferences.AlternativePaths) != 2 || view.Preferences.AlternativePaths[0] != AlternativeTextDescription || view.Preferences.AlternativePaths[1] != AlternativeCallback {
		t.Fatalf("the alternative words and the degraded permitted paths travel with the last outcome, got %v", view.Preferences.AlternativePaths)
	}
	if len(view.Preferences.UsualModalities) != 1 || view.Preferences.UsualModalities[0].MaterialKey != "service_kind" || view.Preferences.UsualModalities[0].Word != UsualModalityMediaPhoto {
		t.Fatalf("the usual modality of the mandatory key derives from the pack data, got %+v", view.Preferences.UsualModalities)
	}
	if !view.Recommendation.Present || len(view.Recommendation.Steps) != 2 || view.Recommendation.Steps[1] != StepHumanHandoff || view.Recommendation.Reason != ReasonMandatoryMissing {
		t.Fatalf("the recommendation projects the plan words and the reason word of the LAST outcome, got %+v", view.Recommendation)
	}
	if len(view.Recommendation.Checks) != 1 || view.Recommendation.Checks[0].MaterialKey != "service_kind" || view.Recommendation.Checks[0].State != MandatoryStateAsk {
		t.Fatalf("the mandatory checks project the plan material, got %+v", view.Recommendation.Checks)
	}
	if view.Recommendation.AckDeadline != request.AckDeadline || !view.Recommendation.DeadlineAt.After(now) {
		t.Fatalf(" the recommendation deadline is the open handoff round, got %+v", view.Recommendation)
	}
	if view.Recommendation.Mode != ModeHumanRequested || view.Recommendation.ModeVersion != 2 {
		t.Fatalf("the recommendation constraints are the effective mode words, got %+v", view.Recommendation)
	}
	if !view.Summary.Present || view.Summary.SummaryNo != 1 || len(view.Summary.Claims) != 3 {
		t.Fatalf("the summary block derives the last row with the claim views, got %+v", view.Summary)
	}
	if view.Summary.Staleness.Word != ContextStalenessStale || view.Summary.Staleness.NewEvents != 2 || !view.Summary.Staleness.OwnerChanged {
		t.Fatalf("the summary staleness compares the watermark with the current counters, got %+v", view.Summary.Staleness)
	}
	for _, claim := range view.Summary.Claims {
		if claim.RefType != "" && !ClaimRefTypeInDictionary(claim.RefType) {
			t.Fatalf("the claim view words belong to the closed dictionary, got %q", claim.RefType)
		}
	}
}

// TestDeriveDecisionContextEmptyControlBranch is the control branch of
// the assembly: the empty material derives the empty view — no block
// panics, every absent block stays absent, and the derivation is
// deterministic (the read-time assembly of the same material twice
// yields the same words).
func TestDeriveDecisionContextEmptyControlBranch(t *testing.T) {
	view := DeriveDecisionContext(DecisionContextInput{Now: time.Now()})
	if view.Handoff.Present || view.Mode.Present || view.Owner.Present || view.Recommendation.Present || view.Summary.Present {
		t.Fatalf("the empty material derives no present block, got %+v", view)
	}
	if len(view.WorkItems) != 0 || len(view.Observations) != 0 || len(view.Deliveries) != 0 || len(view.Commitments) != 0 {
		t.Fatalf("the empty material derives no rows, got %+v", view)
	}
	if len(view.Facts.Confirmed) != 0 || len(view.Facts.Unconfirmed) != 0 || len(view.Facts.Conflicting) != 0 {
		t.Fatalf("the empty material derives no fact groups, got %+v", view.Facts)
	}
	if view.Assurance.IdentityLevel != AssuranceNone || view.Assurance.AuthorityLevel != AssuranceNone {
		t.Fatalf("the absent verification is the honest none (it never becomes a confirmed identity), got %+v", view.Assurance)
	}

	input := DecisionContextInput{
		Now:       time.Now(),
		ModeTerms: []ModeTerm{contextModeTerm(1, ModeSales)},
	}
	first := DeriveDecisionContext(input)
	second := DeriveDecisionContext(input)
	if first.Mode.Mode != second.Mode.Mode || first.Mode.ModeVersion != second.Mode.ModeVersion {
		t.Fatalf("the assembly is deterministic (the same material derives the same words)")
	}
}

// TestDeriveDecisionContextReadTimeControlBranch pins the read-time
// truth of the derivations: the freshness word moves with the clock over
// the SAME stored row (the derivation, never a stored status), and the
// stored row never mutates.
func TestDeriveDecisionContextReadTimeControlBranch(t *testing.T) {
	row := contextObservation(1, MaterialFieldPrice, ObservationStatusOk, time.Now())
	observedAt := row.ObservedAt
	fresh := DeriveDecisionContext(DecisionContextInput{Now: observedAt.Add(10 * time.Second), Observations: []Observation{row}})
	stale := DeriveDecisionContext(DecisionContextInput{Now: observedAt.Add(400 * time.Second), Observations: []Observation{row}})
	if len(fresh.Observations) != 1 || fresh.Observations[0].FreshnessWord != ObservationStateFresh {
		t.Fatalf("the same row derives fresh inside the window, got %+v", fresh.Observations)
	}
	if len(stale.Observations) != 1 || stale.Observations[0].FreshnessWord != ObservationStateStale {
		t.Fatalf("the same row derives stale past the window — the derivation is read-time, got %+v", stale.Observations)
	}
	if !row.ObservedAt.Equal(observedAt) {
		t.Fatalf("the stored row never mutates through the derivation")
	}
}

// TestSummaryOutboxPerimeter pins the outbox perimeter: the payload
// carries the identifiers, the kind word and the claim counters — the
// summary TEXT and the claims NEVER enter the envelope; the dedup key
// names the record; the event type word belongs to the bounded
// vocabulary.
func TestSummaryOutboxPerimeter(t *testing.T) {
	if OutboxEventSummaryRecorded != "case.summary_recorded" || OutboxEventSummaryRecordedV1 != 1 {
		t.Fatalf("the outbox event type is the bounded vocabulary word case.summary_recorded v1")
	}
	summary := recordedSummaryOf()
	linked, disputed := ClaimCountersOf(summary.Claims)
	if linked != 2 || disputed != 1 {
		t.Fatalf("the claim counters count the linked and the disputed claims, got %d/%d", linked, disputed)
	}
	payload := SummaryRecordedPayload{
		TenantID: summary.TenantID, CaseID: summary.CaseID, SummaryNo: summary.SummaryNo,
		Kind: summary.Kind, ClaimCount: len(summary.Claims), LinkedClaims: linked, DisputedClaims: disputed,
		ActorID: summary.ActorID, OccurredAt: time.Now(),
	}
	rendered, err := SummaryRecordedPayloadJSON(payload)
	if err != nil {
		t.Fatalf("the payload renders the canonical JSON, got %v", err)
	}
	text := string(rendered)
	for _, key := range []string{"tenant_id", "case_id", "summary_no", "kind", "claim_count", "linked_claims", "disputed_claims", "actor_id"} {
		if !strings.Contains(text, "\""+key+"\"") {
			t.Fatalf("the payload carries the key %s, got %s", key, text)
		}
	}
	if strings.Contains(text, summaryTextOf) || strings.Contains(text, "Клиент назвал") {
		t.Fatalf("the summary text NEVER enters the outbox envelope, got %s", text)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rendered, &decoded); err != nil {
		t.Fatalf("the payload is the valid JSON, got %v", err)
	}
	if _, carries := decoded["text"]; carries {
		t.Fatalf("the payload carries no text field")
	}
	if _, carries := decoded["claims"]; carries {
		t.Fatalf("the payload carries no claims field")
	}
	key := DedupKeySummaryRecorded(summary.CaseID, summary.SummaryNo)
	if !strings.HasPrefix(key, "case.case_summary:") || !strings.Contains(key, summary.CaseID.String()) || !strings.HasSuffix(key, ":1") {
		t.Fatalf("the dedup key names the case and the summary number, got %s", key)
	}
}
