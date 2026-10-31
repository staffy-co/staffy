package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// correctionDistinctID builds a distinct identifier of the unit matrix
// (the uniqueness proofs need more than one value — newID is a constant).
func correctionDistinctID(n int) id.UUID {
	value, err := id.Parse(fmt.Sprintf("019a2f4c-1000-7000-8000-%012d", n))
	if err != nil {
		panic(err)
	}
	return value
}

// The Story 2.15 unit matrix (the domain half): the dictionaries of the
// slice (the outcome words, the evidence source kinds, the material
// limits and the source-id discipline of the applied pour), the record
// guard (the version pin, the summary link, the proposed value under the
// fact material discipline, the byte-precision anti-hallucination span
// probe and the mandatory reason), the times guard (the 48h provider
// skew of the 2.2 discipline), the matrix-gate outcome derivation (the
// three matrix fields dispute, every other key pours), the read-time
// state derivation (the last correction speaks, the empty control branch
// is the zero view) and the outbox perimeter (the identifiers, the words
// and the span counter — the proposed value and the spans never travel).

// eventTextOf is the accepted-event text of the fixtures: the span of
// the correction lives inside it byte-for-byte.
const eventTextOf = "Клиент сказал: доставка нужна к пятнице, адрес Казань, улица Кремлёвская, 1."

// correctionValueOf builds a canonical fact-material value of the
// fixtures.
func correctionValueOf(raw string) []byte {
	return []byte(raw)
}

// correctionOf builds one correction row of the fixtures.
func correctionOf(outcome string, no int64, summaryNo int64, spans []string) CorrectionRecord {
	return CorrectionRecord{
		TenantID: newID(), CaseID: newID(), CorrectionID: newID(),
		CorrectionNo: no, FactID: newID(), VersionNo: 1,
		SummaryNo:     summaryNo,
		ProposedValue: correctionValueOf(`"к пятнице"`),
		Spans:         spans,
		Outcome:       outcome,
		ActorID:       newID(), Channel: "operator",
		Reason:     "оператор сверил источник",
		ObservedAt: time.Now().Add(-time.Minute),
		ReceivedAt: time.Now(),
		CreatedAt:  time.Now(),
	}
}

// TestCorrectionDictionariesClosed pins the closure of the slice
// dictionaries: the outcome words, the evidence source kinds, the
// material limits and the source-id discipline of the applied pour.
func TestCorrectionDictionariesClosed(t *testing.T) {
	t.Run("outcome dictionary is exactly applied and disputed", func(t *testing.T) {
		if len(CorrectionOutcomes) != 2 || CorrectionOutcomes[0] != CorrectionOutcomeApplied || CorrectionOutcomes[1] != CorrectionOutcomeDisputed {
			t.Fatalf("the outcome dictionary is exactly {applied, disputed}, got %v", CorrectionOutcomes)
		}
		if !CorrectionOutcomeInDictionary("applied") || !CorrectionOutcomeInDictionary("disputed") {
			t.Fatalf("both outcome words belong to the dictionary")
		}
		for _, foreign := range []string{"confirmed", "rejected", "cancelled", "refused", ""} {
			if CorrectionOutcomeInDictionary(foreign) {
				t.Fatalf("the foreign outcome word %q must not belong to the dictionary", foreign)
			}
		}
	})

	t.Run("source kinds are event and summary", func(t *testing.T) {
		if CorrectionSourceKindOf(0) != CorrectionSourceEvent {
			t.Fatalf("summary link 0 is the event evidence")
		}
		if CorrectionSourceKindOf(1) != CorrectionSourceSummary || CorrectionSourceKindOf(7) != CorrectionSourceSummary {
			t.Fatalf("summary link >=1 is the summary evidence")
		}
	})

	t.Run("material limits mirror the fixed dictionaries", func(t *testing.T) {
		if MaxCorrectionValueBytes != MaxFactValueBytes {
			t.Fatalf("the proposed value follows the fact value bound (2048 canonical bytes)")
		}
		if MaxCorrectionSpans != 8 || MaxCorrectionSpanRunes != 200 {
			t.Fatalf("the span discipline is <=8 spans of <=200 runes (2.8)")
		}
		if MinCorrectionReasonRunes != 1 || MaxCorrectionReasonRunes != 200 {
			t.Fatalf("the reason is the mandatory 1..200 runes")
		}
	})

	t.Run("source id of the pour holds the 8..200 fact discipline", func(t *testing.T) {
		correctionID := newID()
		sourceID := CorrectionFactSourceID(correctionID)
		if !strings.HasPrefix(sourceID, "correction:") {
			t.Fatalf("the pour source id carries the correction prefix")
		}
		if len(sourceID) < 8 || len(sourceID) > 200 {
			t.Fatalf("the pour source id fits the GuardRecordFact 8..200 bound, got %d", len(sourceID))
		}
		current := WorkItemCurrent{State: WorkItemStateUntriaged}
		if err := GuardRecordFact(current, FactSourceOperator, sourceID); err != nil {
			t.Fatalf("the composed source id passes the fact write guard, got %v", err)
		}
		if sourceID == CorrectionFactSourceID(correctionDistinctID(42)) {
			t.Fatalf("the pour source id is unique per correction row")
		}
	})
}

// TestGuardRecordCorrection pins the write guard: the version pin, the
// summary link bound, the proposed value under the fact material
// discipline, the span bounds with the byte-precision probe and the
// mandatory reason.
func TestGuardRecordCorrection(t *testing.T) {
	validValue := correctionValueOf(`"к пятнице"`)
	validSpan := "доставка нужна к пятнице"
	validReason := "клиент назвал срок в событии"

	t.Run("valid material passes", func(t *testing.T) {
		err := GuardRecordCorrection(3, 3, 0, validValue, []string{validSpan}, []string{eventTextOf}, validReason)
		if err != nil {
			t.Fatalf("the honest correction material passes, got %v", err)
		}
	})

	t.Run("version pin bounds refuse", func(t *testing.T) {
		for _, pin := range []int64{0, -1, 4} {
			if err := GuardRecordCorrection(3, pin, 0, validValue, []string{validSpan}, []string{eventTextOf}, validReason); err == nil {
				t.Fatalf("the pin %d outside [1..3] refuses", pin)
			}
		}
		if err := GuardRecordCorrection(0, 1, 0, validValue, []string{validSpan}, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("a fact without versions refuses")
		}
		if err := GuardRecordCorrection(3, 1, 0, validValue, []string{validSpan}, []string{eventTextOf}, validReason); err != nil {
			t.Fatalf("the exact lower pin boundary passes, got %v", err)
		}
	})

	t.Run("summary link bound refuses the negative", func(t *testing.T) {
		if err := GuardRecordCorrection(3, 2, -1, validValue, []string{validSpan}, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("the negative summary link refuses (0 = no summary; >=1 = a summary of this case)")
		}
		if err := GuardRecordCorrection(3, 2, 1, validValue, []string{"Срок доставки клиенту не назван."}, []string{summaryTextOf}, validReason); err != nil {
			t.Fatalf("the summary link >=1 passes the domain guard (the belonging is the store guard), got %v", err)
		}
	})

	t.Run("proposed value follows the fact material discipline", func(t *testing.T) {
		if err := GuardRecordCorrection(3, 2, 0, []byte("не JSON"), []string{validSpan}, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("a non-JSON proposed value refuses")
		}
		long := `"` + strings.Repeat("а", MaxCorrectionValueBytes) + `"`
		if err := GuardRecordCorrection(3, 2, 0, []byte(long), []string{validSpan}, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("a proposed value over the 2048 canonical bytes refuses")
		}
		boundary := `"` + strings.Repeat("a", MaxCorrectionValueBytes-2) + `"`
		if err := GuardRecordCorrection(3, 2, 0, []byte(boundary), []string{validSpan}, []string{eventTextOf}, validReason); err != nil {
			t.Fatalf("the exact 2048 canonical byte boundary passes, got %v", err)
		}
		if err := GuardRecordCorrection(3, 2, 0, []byte(`{"срок":1e10}`), []string{validSpan}, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("the exponential number literal refuses (the digit-form discipline of the fact values)")
		}
	})

	t.Run("span bounds refuse", func(t *testing.T) {
		nine := make([]string, 0, MaxCorrectionSpans+1)
		for i := 0; i <= MaxCorrectionSpans; i++ {
			nine = append(nine, validSpan)
		}
		if err := GuardRecordCorrection(3, 2, 0, validValue, nine, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("9 spans refuse (<=8)")
		}
		if err := GuardRecordCorrection(3, 2, 0, validValue, nine[:MaxCorrectionSpans], []string{eventTextOf}, validReason); err != nil {
			t.Fatalf("the exact 8-span boundary passes, got %v", err)
		}
		if err := GuardCorrectionSpans([]string{""}, []string{eventTextOf}); err == nil {
			t.Fatalf("an empty span refuses (1..200 runes)")
		}
		if err := GuardCorrectionSpans([]string{strings.Repeat("а", MaxCorrectionSpanRunes+1)}, []string{strings.Repeat("а", 400)}); err == nil {
			t.Fatalf("a 201-rune span refuses")
		}
	})

	t.Run("spans are optional evidence", func(t *testing.T) {
		if err := GuardRecordCorrection(3, 2, 0, validValue, nil, []string{eventTextOf}, validReason); err != nil {
			t.Fatalf("a correction without spans passes (0..8, the reason is the mandatory why), got %v", err)
		}
	})

	t.Run("span absent from the source refuses", func(t *testing.T) {
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{"Клиент отказался от доставки"}, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("a fabricated span refuses (the anti-hallucination probe)")
		}
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{validSpan}, nil, validReason); err == nil {
			t.Fatalf("a cited span with no source material refuses honestly")
		}
	})

	t.Run("byte precision is not a paraphrase", func(t *testing.T) {
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{"ДОСТАВКА НУЖНА К ПЯТНИЦЕ"}, []string{eventTextOf}, validReason); err == nil {
			t.Fatalf("the case difference is a fabrication — the span must be byte-for-byte")
		}
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{"Казань, улица Кремлёвская, 1"}, []string{eventTextOf}, validReason); err != nil {
			t.Fatalf("an inner span of the event passes, got %v", err)
		}
	})

	t.Run("span may cite the second source text", func(t *testing.T) {
		second := "Второе событие: клиент подтвердил срок отдельным сообщением."
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{"клиент подтвердил срок"}, []string{eventTextOf, second}, validReason); err != nil {
			t.Fatalf("a span present in the second source text passes, got %v", err)
		}
	})

	t.Run("reason bounds refuse", func(t *testing.T) {
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{validSpan}, []string{eventTextOf}, "   "); err == nil {
			t.Fatalf("a whitespace reason refuses (the act is conscious)")
		}
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{validSpan}, []string{eventTextOf}, strings.Repeat("а", MaxCorrectionReasonRunes+1)); err == nil {
			t.Fatalf("a 201-rune reason refuses")
		}
		if err := GuardRecordCorrection(3, 2, 0, validValue, []string{validSpan}, []string{eventTextOf}, strings.Repeat("а", MaxCorrectionReasonRunes)); err != nil {
			t.Fatalf("the exact 200-rune boundary passes, got %v", err)
		}
	})
}

// TestGuardCorrectionTimes pins the 2.2 time discipline: the observed
// time is mandatory and never claims more than the 48h provider skew
// ahead of the receive time.
func TestGuardCorrectionTimes(t *testing.T) {
	now := time.Now()

	t.Run("the honest times pass", func(t *testing.T) {
		if err := GuardCorrectionTimes(now.Add(-time.Hour), now); err != nil {
			t.Fatalf("the observed time in the past passes, got %v", err)
		}
		if err := GuardCorrectionTimes(now.Add(48*time.Hour), now); err != nil {
			t.Fatalf("the exact 48h skew boundary passes, got %v", err)
		}
	})

	t.Run("the missing times refuse", func(t *testing.T) {
		if err := GuardCorrectionTimes(time.Time{}, now); err == nil {
			t.Fatalf("a zero observed time refuses")
		}
		if err := GuardCorrectionTimes(now, time.Time{}); err == nil {
			t.Fatalf("a zero received time refuses")
		}
	})

	t.Run("the future skew refuses", func(t *testing.T) {
		if err := GuardCorrectionTimes(now.Add(48*time.Hour+time.Minute), now); err == nil {
			t.Fatalf("the observed time beyond the 48h skew refuses")
		}
	})
}

// TestDeriveCorrectionOutcome pins the matrix gate by the three matrix
// fields: price, availability and order_status dispute (the Owning
// System owns the value — the pour never happens); every other fact key
// pours (applied).
func TestDeriveCorrectionOutcome(t *testing.T) {
	for _, matrixKey := range []string{MaterialFieldPrice, MaterialFieldAvailability, MaterialFieldOrderStatus} {
		if got := DeriveCorrectionOutcome(matrixKey); got != CorrectionOutcomeDisputed {
			t.Fatalf("the matrix field %s disputes (the system value is never rewritten), got %s", matrixKey, got)
		}
	}
	for _, otherKey := range []string{"delivery_address", "deadline", "phone_number", "price_comment"} {
		if got := DeriveCorrectionOutcome(otherKey); got != CorrectionOutcomeApplied {
			t.Fatalf("the non-matrix key %s pours through the fact discipline, got %s", otherKey, got)
		}
	}
}

// TestDeriveCorrectionState pins the read-time state derivation: the
// last correction speaks (the store order trust), the disputed outcome
// carries the honest boundary text, the evidence source kind follows the
// summary link, and the empty control branch is the zero view (the
// markers appear only with the material).
func TestDeriveCorrectionState(t *testing.T) {
	t.Run("no corrections is the zero view", func(t *testing.T) {
		view := DeriveCorrectionState(nil)
		if view.HasCorrection || view.CorrectionMarkerText() != "" || view.CorrectionSourceText() != "" {
			t.Fatalf("the control branch renders byte-identically — no markers without material, got %+v", view)
		}
	})

	t.Run("the last correction speaks", func(t *testing.T) {
		first := correctionOf(CorrectionOutcomeApplied, 1, 0, []string{"доставка нужна к пятнице"})
		second := correctionOf(CorrectionOutcomeDisputed, 2, 0, []string{"доставка нужна к пятнице"})
		view := DeriveCorrectionState([]CorrectionRecord{first, second})
		if !view.HasCorrection || view.CorrectionNo != 2 || view.Outcome != CorrectionOutcomeDisputed {
			t.Fatalf("the last correction (№2, disputed) speaks, got %+v", view)
		}
		marker := view.CorrectionMarkerText()
		if marker != "оспорено (значение системы не изменено)" {
			t.Fatalf("the disputed marker carries the honest boundary, got %q", marker)
		}
	})

	t.Run("the applied marker and the source line", func(t *testing.T) {
		applied := correctionOf(CorrectionOutcomeApplied, 1, 0, []string{"доставка нужна к пятнице"})
		view := DeriveCorrectionState([]CorrectionRecord{applied})
		if view.CorrectionMarkerText() != "исправлено" {
			t.Fatalf("the applied marker is «исправлено», got %q", view.CorrectionMarkerText())
		}
		if view.CorrectionSourceText() != "источник исправления: событие" {
			t.Fatalf("the event evidence line, got %q", view.CorrectionSourceText())
		}
		if view.SourceKind != CorrectionSourceEvent || len(view.Spans) != 1 || view.Spans[0] != "доставка нужна к пятнице" {
			t.Fatalf("the view carries the spans and the source kind, got %+v", view)
		}
	})

	t.Run("the summary link derives the summary evidence line", func(t *testing.T) {
		fromSummary := correctionOf(CorrectionOutcomeApplied, 1, 1, []string{"Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1."})
		view := DeriveCorrectionState([]CorrectionRecord{fromSummary})
		if view.SourceKind != CorrectionSourceSummary || view.CorrectionSourceText() != "источник исправления: сводка" {
			t.Fatalf("the summary link derives the summary evidence line, got %+v", view)
		}
	})

	t.Run("the derivation never mutates the rows", func(t *testing.T) {
		row := correctionOf(CorrectionOutcomeApplied, 1, 0, []string{"доставка нужна к пятнице"})
		before := row.Outcome
		_ = DeriveCorrectionState([]CorrectionRecord{row})
		if row.Outcome != before || row.CorrectionNo != 1 {
			t.Fatalf("the read-time derivation never mutates the correction rows")
		}
	})
}

// TestCorrectionOutboxPerimeter pins the outbox perimeter: the
// identifiers, the dictionary words and the span counter travel; the
// proposed value and the spans NEVER enter the envelope.
func TestCorrectionOutboxPerimeter(t *testing.T) {
	correction := correctionOf(CorrectionOutcomeApplied, 2, 1, []string{"доставка нужна к пятнице", "Казань, улица Кремлёвская, 1"})
	payload := CorrectionRecordedPayload{
		TenantID: correction.TenantID, CaseID: correction.CaseID,
		CorrectionID: correction.CorrectionID, CorrectionNo: correction.CorrectionNo,
		FactID: correction.FactID, VersionNo: correction.VersionNo,
		SummaryNo: correction.SummaryNo, Outcome: correction.Outcome,
		SpanCount: len(correction.Spans), ActorID: correction.ActorID,
		OccurredAt: correction.CreatedAt,
	}
	raw, err := CorrectionRecordedPayloadJSON(payload)
	if err != nil {
		t.Fatalf("the payload renders, got %v", err)
	}

	t.Run("the identifiers and the words travel", func(t *testing.T) {
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("the payload is canonical JSON, got %v", err)
		}
		for _, key := range []string{"tenant_id", "case_id", "correction_id", "correction_no", "fact_id", "version_no", "summary_no", "outcome", "span_count", "actor_id", "occurred_at"} {
			if _, ok := decoded[key]; !ok {
				t.Fatalf("the key %s of the identifier perimeter is present", key)
			}
		}
		if decoded["outcome"] != CorrectionOutcomeApplied {
			t.Fatalf("the outcome word travels")
		}
		if decoded["span_count"] != float64(2) {
			t.Fatalf("the span counter travels (the count, never the spans), got %v", decoded["span_count"])
		}
	})

	t.Run("the proposed value and the spans never travel", func(t *testing.T) {
		text := string(raw)
		if strings.Contains(text, "proposed_value") || strings.Contains(text, "доставка нужна к пятнице") {
			t.Fatalf("the proposed value and the span text never enter the outbox envelope")
		}
		if strings.Contains(text, "Казань") {
			t.Fatalf("the span material never leaks into the outbox envelope")
		}
	})

	t.Run("the dedup key carries the case and the number", func(t *testing.T) {
		key := DedupKeyCorrectionRecorded(correction.CaseID, correction.CorrectionNo)
		if !strings.Contains(key, correction.CaseID.String()) || !strings.HasSuffix(key, ":2") {
			t.Fatalf("the dedup key pins the case and the correction number, got %q", key)
		}
	})
}
