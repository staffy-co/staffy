package domain

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 2.8 unit matrix: the dictionaries, the playbook v1 rule table,
// the record guards (the honest refusal texts), the span containment, the
// persuasion-state derivations (the stop words and the acknowledged word)
// and the sales-reply gate branches.

// objectionTestID builds a distinct deterministic id (the newID helper of
// the policy tests stays untouched).
func objectionTestID() id.UUID {
	return id.MustParse("019a2f4c-2000-7000-8000-00000000000a")
}

// itoaDigits renders the decimal text of a small non-negative int (the
// span fixtures of the local tests).
func itoaDigits(value int) string {
	if value == 0 {
		return "0"
	}
	out := ""
	for value > 0 {
		out = string(rune('0'+value%10)) + out
		value /= 10
	}
	return out
}

func TestObjectionDictionariesAreClosed(t *testing.T) {
	// No type, signal or step word appears outside the fixed dictionary
	// (the fail-closed basis of the slice).
	if len(ObjectionTypes) != 3 {
		t.Fatalf("the type dictionary carries three words, got %v", ObjectionTypes)
	}
	if len(ClientSignals) != 4 {
		t.Fatalf("the signal dictionary carries four words, got %v", ClientSignals)
	}
	if len(PlaybookSteps) != 5 {
		t.Fatalf("the step dictionary carries five words, got %v", PlaybookSteps)
	}
	for _, signal := range ClientSignals {
		if !inDictionary(ClientSignals, signal) {
			t.Fatalf("the signal %q must be in its own dictionary", signal)
		}
	}
	if PersuasionThresholdV1 != 2 {
		t.Fatalf("the v1 threshold is two refusals, got %d", PersuasionThresholdV1)
	}
	if ObjectionPlaybookVersion != "objection-playbook-v1" {
		t.Fatalf("the playbook version word is fixed, got %q", ObjectionPlaybookVersion)
	}
}

func TestPlaybookV1CoversTheTypesWithTheOrderedSteps(t *testing.T) {
	// The fixed rule table: every type names its ordered allowed steps
	// (the versioned playbook of AC-4).
	expected := map[string][]string{
		"trust":            {"acknowledge_concern", "present_evidence", "honest_no_answer", "offer_alternative"},
		"price_comparison": {"acknowledge_concern", "present_evidence", "offer_alternative", "honest_no_answer"},
		"general_doubt":    {"acknowledge_concern", "clarify_question", "present_evidence", "honest_no_answer"},
	}
	if len(playbookV1) != len(expected) {
		t.Fatalf("the playbook v1 covers exactly the three types, got %d", len(playbookV1))
	}
	for objectionType, want := range expected {
		steps := PlaybookStepsOf(objectionType)
		if len(steps) != len(want) {
			t.Fatalf("the playbook of %q carries %d steps, want %v", objectionType, len(steps), want)
		}
		for i, step := range want {
			if steps[i] != step {
				t.Fatalf("the step %d of %q must be %q, got %q", i, objectionType, step, steps[i])
			}
			if !PlaybookAllowsStep(objectionType, step) {
				t.Fatalf("the step %q must be allowed for %q", step, objectionType)
			}
		}
	}
	// The step outside the playbook of the type refuses (the honest
	// machine frame); an unknown type allows nothing.
	if PlaybookAllowsStep("trust", PlaybookStepClarifyQuestion) {
		t.Fatal("clarify_question is outside the trust playbook")
	}
	if PlaybookAllowsStep("price_comparison", PlaybookStepClarifyQuestion) {
		t.Fatal("clarify_question is outside the price_comparison playbook")
	}
	if PlaybookAllowsStep("magic_type", PlaybookStepAcknowledgeConcern) {
		t.Fatal("an unknown type allows nothing")
	}
	if steps := PlaybookStepsOf("magic_type"); steps != nil {
		t.Fatalf("an unknown type has no steps, got %v", steps)
	}
	// The copy discipline: the returned slice never aliases the table.
	steps := PlaybookStepsOf(ObjectionTypeTrust)
	steps[0] = "mutated"
	if playbookV1[ObjectionTypeTrust][0] != PlaybookStepAcknowledgeConcern {
		t.Fatal("PlaybookStepsOf returns a copy — the rule table is immutable")
	}
}

func TestGuardObjectionRecordRefusesDishonestMaterial(t *testing.T) {
	// The honest record: the dictionary words with the bounded spans.
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, []string{"Мне кажется дорого"}); err != nil {
		t.Fatalf("the honest record must pass, got %v", err)
	}
	if err := GuardObjectionRecord("magic_type", ClientSignalConcern, []string{"слова"}); err == nil {
		t.Fatal("the type outside the dictionary must refuse")
	}
	if err := GuardObjectionRecord(ObjectionTypeTrust, "magic_signal", []string{"слова"}); err == nil {
		t.Fatal("the signal outside the dictionary must refuse")
	}
	// The refusal names the full dictionary (the machine-readable text).
	err := GuardObjectionRecord("magic_type", ClientSignalConcern, []string{"слова"})
	for _, word := range ObjectionTypes {
		if !strings.Contains(err.Error(), "«"+word+"»") {
			t.Fatalf("the refusal lists the type word %q, got %q", word, err.Error())
		}
	}
	err = GuardObjectionRecord(ObjectionTypeTrust, "magic_signal", []string{"слова"})
	for _, word := range ClientSignals {
		if !strings.Contains(err.Error(), "«"+word+"»") {
			t.Fatalf("the refusal lists the signal word %q, got %q", word, err.Error())
		}
	}
	// The span discipline: at least one, at most eight, non-empty,
	// bounded, no duplicates.
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, nil); err == nil {
		t.Fatal("a record without spans must refuse")
	}
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, []string{}); err == nil {
		t.Fatal("an empty span list must refuse")
	}
	many := make([]string, MaxObjectionSpans+1)
	for i := range many {
		many[i] = "выдержка " + itoaDigits(i)
	}
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, many); err == nil {
		t.Fatal("more than eight spans must refuse")
	}
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, []string{"", "вторая"}); err == nil {
		t.Fatal("an empty span must refuse")
	}
	long := strings.Repeat("а", MaxObjectionSpanRunes+1)
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, []string{long}); err == nil {
		t.Fatal("a span longer than 200 runes must refuse")
	}
	exact := strings.Repeat("а", MaxObjectionSpanRunes)
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, []string{exact}); err != nil {
		t.Fatalf("a span of exactly 200 runes passes, got %v", err)
	}
	if err := GuardObjectionRecord(ObjectionTypeTrust, ClientSignalConcern, []string{"дубль", "дубль"}); err == nil {
		t.Fatal("a duplicate span must refuse")
	}
}

func TestGuardPlaybookStepBoundedByTheDictionary(t *testing.T) {
	if err := GuardPlaybookStep(""); err != nil {
		t.Fatalf("the empty step is the honest absence, got %v", err)
	}
	for _, step := range PlaybookSteps {
		if err := GuardPlaybookStep(step); err != nil {
			t.Fatalf("the dictionary step %q passes, got %v", step, err)
		}
	}
	err := GuardPlaybookStep("magic_step")
	for _, word := range PlaybookSteps {
		if !strings.Contains(err.Error(), "«"+word+"»") {
			t.Fatalf("the refusal lists the step word %q, got %q", word, err.Error())
		}
	}
}

func TestSpanInPayloadIsTheByteExactContainment(t *testing.T) {
	// The honest excerpt is the substring of the client words: an
	// invented excerpt is impossible (AC-4).
	payload := []byte(`{"text":"Мне кажется дорого, у конкурента дешевле"}`)
	if !SpanInPayload("у конкурента дешевле", payload) {
		t.Fatal("the verbatim excerpt is contained")
	}
	if !SpanInPayload("Мне кажется", payload) {
		t.Fatal("a partial-word excerpt of the payload bytes is contained")
	}
	if SpanInPayload("у конкурента дешевле и быстрее", payload) {
		t.Fatal("an extended excerpt is NOT contained")
	}
	if SpanInPayload("выдуманная выдержка", payload) {
		t.Fatal("an invented excerpt is NOT contained")
	}
	// The empty needle is trivially contained by bytes.Contains — the
	// RECORD guard rejects the empty span earlier (the honest boundary
	// of the discipline, verified in the guard test above).
	if !SpanInPayload("", payload) {
		t.Fatal("bytes.Contains trivially contains the empty span — the guard is the boundary")
	}
}

func objectionFixture(no int64, signal string) ObjectionRecord {
	return ObjectionRecord{
		ObjectionID: objectionTestIDOf(no), ObjectionNo: no, ObjectionType: ObjectionTypePriceComparison,
		ClientSignal: signal, PlaybookVersion: ObjectionPlaybookVersion,
	}
}

// objectionTestIDOf builds the distinct deterministic id of one fixture
// record (the acknowledged derivation compares the objection ids).
func objectionTestIDOf(no int64) id.UUID {
	return id.MustParse(fmt.Sprintf("019a2f4c-2000-7000-8000-%012d", no))
}

func TestDeriveObjectionStateCurrentIsTheMaximalNumber(t *testing.T) {
	records := []ObjectionRecord{objectionFixture(1, ClientSignalConcern), objectionFixture(2, ClientSignalRefusal), objectionFixture(3, ClientSignalConcern)}
	state := DeriveObjectionState(records, nil)
	if state.Current == nil || state.Current.ObjectionNo != 3 {
		t.Fatalf("the current objection is the maximal number, got %+v", state.Current)
	}
	if state.Refusals != 1 {
		t.Fatalf("the lifetime refusal count is one, got %d", state.Refusals)
	}
	if state.Stopped {
		t.Fatal("one refusal under the threshold does not stop the persuasion")
	}
}

func TestDeriveObjectionStateStopsByTheStopSignals(t *testing.T) {
	// stop_intent / human_request of the CURRENT objection stop
	// immediately — the FIRST record is enough (AC-7).
	for _, signal := range []string{ClientSignalStopIntent, ClientSignalHumanRequest} {
		state := DeriveObjectionState([]ObjectionRecord{objectionFixture(1, signal)}, nil)
		if !state.Stopped {
			t.Fatalf("the %s signal stops the persuasion immediately", signal)
		}
	}
	// A historical stop signal superseded by a later concern: the
	// current word decides (the mode raise stays the propose_mode
	// command — the record only marks the trigger).
	records := []ObjectionRecord{objectionFixture(1, ClientSignalStopIntent), objectionFixture(2, ClientSignalConcern)}
	state := DeriveObjectionState(records, nil)
	if state.Stopped {
		t.Fatal("the superseded stop signal is not the current word — the derivation reads the current record")
	}
}

func TestDeriveObjectionStateStopsAtTheThreshold(t *testing.T) {
	// The lifetime budget: two refusal records stop the persuasion for
	// good — the count never resets (AC-7 «повторно отказывается в
	// пределах установленного playbook threshold»).
	state := DeriveObjectionState([]ObjectionRecord{objectionFixture(1, ClientSignalRefusal), objectionFixture(2, ClientSignalRefusal)}, nil)
	if state.Refusals != 2 || !state.Stopped {
		t.Fatalf("two refusals stop the persuasion, got refusals=%d stopped=%v", state.Refusals, state.Stopped)
	}
	// A refusal in the history plus a newer concern still counts: the
	// budget is lifetime.
	records := []ObjectionRecord{objectionFixture(1, ClientSignalRefusal), objectionFixture(2, ClientSignalConcern), objectionFixture(3, ClientSignalRefusal)}
	state = DeriveObjectionState(records, nil)
	if state.Refusals != 2 || !state.Stopped {
		t.Fatalf("the lifetime budget counts every refusal, got refusals=%d stopped=%v", state.Refusals, state.Stopped)
	}
}

func TestDeriveObjectionStateAcknowledgedOfTheCurrentObjection(t *testing.T) {
	// The acknowledged word: a linked reply with acknowledge_concern FOR
	// THE CURRENT objection (a new objection acknowledges anew).
	first := objectionFixture(1, ClientSignalConcern)
	second := ObjectionRecord{
		ObjectionID: objectionTestIDOf(2), ObjectionNo: 2, ObjectionType: ObjectionTypeTrust,
		ClientSignal: ClientSignalConcern, PlaybookVersion: ObjectionPlaybookVersion,
	}
	links := []ReplyLink{{ObjectionID: first.ObjectionID, Step: PlaybookStepAcknowledgeConcern}}
	if state := DeriveObjectionState([]ObjectionRecord{first}, links); !state.Acknowledged {
		t.Fatal("the linked acknowledge of the current objection acknowledges it")
	}
	if state := DeriveObjectionState([]ObjectionRecord{first, second}, links); state.Acknowledged {
		t.Fatal("the acknowledge of the OLD objection does not acknowledge the current one")
	}
	// A non-acknowledging linked reply never acknowledges.
	evidence := []ReplyLink{{ObjectionID: first.ObjectionID, Step: PlaybookStepPresentEvidence}}
	if state := DeriveObjectionState([]ObjectionRecord{first}, evidence); state.Acknowledged {
		t.Fatal("only acknowledge_concern acknowledges the objection")
	}
	// No records: no current, no acknowledged, the empty state.
	if state := DeriveObjectionState(nil, links); state.Current != nil || state.Acknowledged || state.Stopped || state.Refusals != 0 {
		t.Fatalf("no objections — the honest empty state, got %+v", state)
	}
}

func TestGuardSalesReplyAgainstObjectionsGateBranches(t *testing.T) {
	concern := objectionFixture(1, ClientSignalConcern)
	// The honest first answer: acknowledge_concern passes.
	if err := GuardSalesReplyAgainstObjections(DeriveObjectionState([]ObjectionRecord{concern}, nil), PlaybookStepAcknowledgeConcern); err != nil {
		t.Fatalf("the first linked answer acknowledges the concern, got %v", err)
	}
	// A non-acknowledging first answer refuses: «сначала признаёт».
	err := GuardSalesReplyAgainstObjections(DeriveObjectionState([]ObjectionRecord{concern}, nil), PlaybookStepPresentEvidence)
	if err == nil || !strings.Contains(err.Error(), "acknowledge_concern") {
		t.Fatalf("the first answer must acknowledge, got %v", err)
	}
	// The acknowledged state: the next playbook step passes — «затем
	// доказательство ИЛИ честное отсутствие ответа» (AC-4).
	links := []ReplyLink{{ObjectionID: concern.ObjectionID, Step: PlaybookStepAcknowledgeConcern}}
	for _, step := range PlaybookStepsOf(concern.ObjectionType) {
		if err := GuardSalesReplyAgainstObjections(DeriveObjectionState([]ObjectionRecord{concern}, links), step); err != nil {
			t.Fatalf("the acknowledged state allows the playbook step %q, got %v", step, err)
		}
	}
	// The step outside the playbook of the type refuses.
	err = GuardSalesReplyAgainstObjections(DeriveObjectionState([]ObjectionRecord{concern}, links), PlaybookStepClarifyQuestion)
	if err == nil || !strings.Contains(err.Error(), "последовательность") {
		t.Fatalf("the step outside the playbook of the type refuses, got %v", err)
	}
	// The step word outside the dictionary refuses.
	if err := GuardSalesReplyAgainstObjections(DeriveObjectionState([]ObjectionRecord{concern}, links), "magic_step"); err == nil {
		t.Fatal("the step outside the dictionary refuses")
	}
	// No step against a current objection: the discipline is mandatory.
	err = GuardSalesReplyAgainstObjections(DeriveObjectionState([]ObjectionRecord{concern}, nil), "")
	if err == nil || !strings.Contains(err.Error(), "дисциплина") {
		t.Fatalf("the absent step against a current objection refuses, got %v", err)
	}
	// A foreign step without any objection refuses (the old path stays
	// byte-identical only with the EMPTY step).
	err = GuardSalesReplyAgainstObjections(DeriveObjectionState(nil, nil), PlaybookStepPresentEvidence)
	if err == nil || !strings.Contains(err.Error(), "оставьте поле пустым") {
		t.Fatalf("the foreign step without an objection refuses, got %v", err)
	}
	// No objection + the empty step: the byte-identical old path passes
	// (the control branch of the frozen matrix).
	if err := GuardSalesReplyAgainstObjections(DeriveObjectionState(nil, nil), ""); err != nil {
		t.Fatalf("no objection with the empty step is the old path, got %v", err)
	}
}

func TestGuardSalesReplyAgainstObjectionsStoppedRefusesWithTheEndings(t *testing.T) {
	// The stopped persuasion (the threshold): the refusal is the typed
	// error with the budget words and the allowed endings.
	stopped := DeriveObjectionState([]ObjectionRecord{objectionFixture(1, ClientSignalRefusal), objectionFixture(2, ClientSignalRefusal)}, nil)
	err := GuardSalesReplyAgainstObjections(stopped, PlaybookStepAcknowledgeForTest())
	if err == nil {
		t.Fatal("the stopped persuasion refuses the sales answer")
	}
	var typed ObjectionStoppedError
	if !asObjectionStopped(err, &typed) {
		t.Fatalf("the refusal is the ObjectionStoppedError, got %T", err)
	}
	if typed.Refusals != 2 || typed.Threshold != PersuasionThresholdV1 {
		t.Fatalf("the refusal carries the budget words, got refusals=%d threshold=%d", typed.Refusals, typed.Threshold)
	}
	if typed.CurrentSignal != ClientSignalRefusal {
		t.Fatalf("the refusal carries the current signal word, got %q", typed.CurrentSignal)
	}
	for _, ending := range []string{"вежливое завершение", "запрос приватности", "передача человеку"} {
		if !strings.Contains(typed.Reason, ending) {
			t.Fatalf("the refusal names the allowed ending %q, got %q", ending, typed.Reason)
		}
	}
	// The immediate stop signal (stop_intent of the current record).
	immediate := DeriveObjectionState([]ObjectionRecord{objectionFixture(1, ClientSignalStopIntent)}, nil)
	err = GuardSalesReplyAgainstObjections(immediate, PlaybookStepAcknowledgeForTest())
	if err == nil {
		t.Fatal("the stop signal refuses the sales answer immediately")
	}
	if !asObjectionStopped(err, &typed) || typed.CurrentSignal != ClientSignalStopIntent {
		t.Fatalf("the immediate stop refusal carries the signal word, got %+v", typed)
	}
	if typed.Refusals != 0 {
		t.Fatalf("the immediate stop fires before the budget, got refusals=%d", typed.Refusals)
	}
}

// PlaybookStepAcknowledgeForTest is the honest step word of the
// stopped-probe answers (the stop fires BEFORE the step discipline).
func PlaybookStepAcknowledgeForTest() string { return PlaybookStepAcknowledgeConcern }

// asObjectionStopped resolves the typed stop error (the errors.As helper
// of the local tests).
func asObjectionStopped(err error, target *ObjectionStoppedError) bool {
	typed, ok := err.(ObjectionStoppedError)
	if ok {
		*target = typed
	}
	return ok
}

func TestObjectionRecordedPayloadCarriesNoSpanText(t *testing.T) {
	// The outbox envelope carries the identifiers, the dictionary words
	// and the monotonic number only — the SPAN TEXT never enters (the
	// client words already live in the sealed payload of 2-1).
	payload, err := ObjectionRecordedPayloadJSON(ObjectionRecordedPayload{
		TenantID: objectionTestID(), CaseID: objectionTestID(), WorkItemID: objectionTestID(),
		ObjectionID: objectionTestID(), ObjectionNo: 1, TurnRef: objectionTestID(),
		ObjectionType: ObjectionTypePriceComparison, ClientSignal: ClientSignalConcern,
		PlaybookVersion: ObjectionPlaybookVersion, ActorID: objectionTestID(),
		OccurredAt: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if strings.Contains(text, "span") {
		t.Fatalf("the envelope never carries the span text, got %s", text)
	}
	for _, word := range []string{"objection_no", "turn_ref", "objection_type", "client_signal", "playbook_version"} {
		if !strings.Contains(text, word) {
			t.Fatalf("the envelope carries the metadata word %q, got %s", word, text)
		}
	}
	if DedupKeyObjectionRecorded(objectionTestID()) != "case.objection_recorded:"+objectionTestID().String() {
		t.Fatal("the dedup key is the event type with the objection id")
	}
}
