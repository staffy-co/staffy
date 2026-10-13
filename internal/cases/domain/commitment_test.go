package domain

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.9 unit matrix: the dictionaries, the MaterialTerms v1
// canonicalization and guards, the price term discipline, the readiness
// derivation, the confirmation validity derivation (the reserved words
// without a producer), the content binding token (the domain separation
// and the exact-version binding), the evidence guards (the invented
// consent impossibility) and the additive confirmed ladder of 2.2.

// commitmentTestID builds a distinct deterministic id.
func commitmentTestID() id.UUID {
	return id.MustParse("019a2f4c-2900-7000-8000-000000000009")
}

// validTerms builds the complete valid material of the local tests: the
// auto-parts scope, two service items, the exact price, the payee, the
// warranty role, the next step and the consequences.
func validTerms() MaterialTerms {
	return MaterialTerms{
		Scope:        "Замена масла и фильтров, легковой контур",
		ServiceItems: []string{"Масло 5W-30 4л", "Фильтр масляный"},
		Price:        CommitmentPrice{Kind: CommitmentPriceKindAmount, AmountCents: 450000},
		Payee:        "ООО «Запчасть»",
		WarrantyRole: "Гарантия на работы 30 дней",
		NextStep:     "Клиент привозит автомобиль в согласованный слот",
		Consequences: "Отмена менее чем за 2 часа — счёт за слот",
	}
}

func TestCommitmentDictionariesAreClosed(t *testing.T) {
	// No status, term, action, method, outcome or validity word appears
	// outside its fixed dictionary (the fail-closed basis of the slice).
	if len(CommitmentStatuses) != 2 {
		t.Fatalf("the status dictionary carries two words, got %v", CommitmentStatuses)
	}
	if len(CommitmentMandatoryTerms) != 7 {
		t.Fatalf("the mandatory term dictionary carries seven words, got %v", CommitmentMandatoryTerms)
	}
	if len(CommitmentActions) != 3 {
		t.Fatalf("the action dictionary carries three words, got %v", CommitmentActions)
	}
	if len(CommitmentMethods) != 1 || CommitmentMethods[0] != CommitmentMethodExplicitText {
		t.Fatalf("the method dictionary carries one explicit_text word, got %v", CommitmentMethods)
	}
	if len(CommitmentDeclineOutcomes) != 2 {
		t.Fatalf("the decline outcome dictionary carries two words, got %v", CommitmentDeclineOutcomes)
	}
	for _, word := range CommitmentReadinessWords {
		if !inDictionary(CommitmentReadinessWords, word) {
			t.Fatalf("the readiness word %q must be in its own dictionary", word)
		}
	}
	// The reserved validity words live in the dictionary without a
	// producer (the honest reservation of the slice — the precedent of
	// the reserved mandatory_missing word of 2.6).
	for _, reserved := range []string{ConfirmationValidityRevoked, ConfirmationValidityConsumed} {
		if !inDictionary(ConfirmationValidityWords, reserved) {
			t.Fatalf("the reserved word %q must stay in the validity dictionary", reserved)
		}
	}
	// The observation-backed rule table v1 names the price term only.
	if len(CommitmentObservationBackedTerms) != 1 || !ObservationBackedTerm(CommitmentTermPrice) {
		t.Fatalf("the observation-backed rule table v1 is {price}, got %v", CommitmentObservationBackedTerms)
	}
	if ObservationBackedTerm(CommitmentTermScope) {
		t.Fatalf("the scope term is not observation-backed")
	}
}

func TestGuardCommitmentPrice(t *testing.T) {
	cases := []struct {
		name  string
		price CommitmentPrice
	}{
		{"zero amount", CommitmentPrice{Kind: CommitmentPriceKindAmount}},
		{"negative amount", CommitmentPrice{Kind: CommitmentPriceKindAmount, AmountCents: -1}},
		{"amount with range", CommitmentPrice{Kind: CommitmentPriceKindAmount, AmountCents: 100, MinCents: 1, MaxCents: 2}},
		{"zero range min", CommitmentPrice{Kind: CommitmentPriceKindRange, MinCents: 0, MaxCents: 100}},
		{"collapsed range", CommitmentPrice{Kind: CommitmentPriceKindRange, MinCents: 500, MaxCents: 500}},
		{"inverted range", CommitmentPrice{Kind: CommitmentPriceKindRange, MinCents: 700, MaxCents: 500}},
		{"range with amount", CommitmentPrice{Kind: CommitmentPriceKindRange, AmountCents: 100, MinCents: 1, MaxCents: 2}},
		{"unknown kind", CommitmentPrice{Kind: "approximate"}},
	}
	for _, testCase := range cases {
		if err := GuardCommitmentPrice(testCase.price); err == nil {
			t.Fatalf("%s: the price guard must refuse %v", testCase.name, testCase.price)
		}
	}
	if err := GuardCommitmentPrice(CommitmentPrice{Kind: CommitmentPriceKindAmount, AmountCents: 450000}); err != nil {
		t.Fatalf("the exact amount must pass: %v", err)
	}
	if err := GuardCommitmentPrice(CommitmentPrice{Kind: CommitmentPriceKindRange, MinCents: 400000, MaxCents: 600000}); err != nil {
		t.Fatalf("the honest range must pass: %v", err)
	}
}

func TestGuardMaterialTerms(t *testing.T) {
	empty := map[string]func(MaterialTerms) MaterialTerms{
		"scope":        func(m MaterialTerms) MaterialTerms { m.Scope = " "; return m },
		"payee":        func(m MaterialTerms) MaterialTerms { m.Payee = ""; return m },
		"warranty":     func(m MaterialTerms) MaterialTerms { m.WarrantyRole = " "; return m },
		"next step":    func(m MaterialTerms) MaterialTerms { m.NextStep = ""; return m },
		"consequences": func(m MaterialTerms) MaterialTerms { m.Consequences = ""; return m },
	}
	for name, mutate := range empty {
		if err := GuardMaterialTerms(mutate(validTerms())); err == nil {
			t.Fatalf("the empty %s term must refuse", name)
		}
	}
	noItems := validTerms()
	noItems.ServiceItems = nil
	if err := GuardMaterialTerms(noItems); err == nil {
		t.Fatalf("the empty service item list must refuse")
	}
	dupItems := validTerms()
	dupItems.ServiceItems = []string{"Масло 5W-30 4л", "Масло 5W-30 4л"}
	if err := GuardMaterialTerms(dupItems); err == nil {
		t.Fatalf("the duplicate service items must refuse")
	}
	oversize := validTerms()
	oversize.Consequences = strings.Repeat("и", MaxCommitmentTermRunes+1)
	if err := GuardMaterialTerms(oversize); err == nil {
		t.Fatalf("the oversize term must refuse")
	}
	if err := GuardMaterialTerms(validTerms()); err != nil {
		t.Fatalf("the complete valid material must pass: %v", err)
	}
}

func TestGuardUnknownsActionSetAndLimits(t *testing.T) {
	if err := GuardMaterialUnknowns([]string{"brand"}); err == nil {
		t.Fatalf("a non-mandatory unknown word must refuse")
	}
	if err := GuardMaterialUnknowns([]string{CommitmentTermPrice, CommitmentTermPrice}); err == nil {
		t.Fatalf("the duplicate unknown words must refuse")
	}
	if err := GuardMaterialUnknowns([]string{CommitmentTermPrice}); err != nil {
		t.Fatalf("the price unknown word is the honest hole: %v", err)
	}
	if err := GuardMaterialUnknowns(nil); err != nil {
		t.Fatalf("the empty unknown list must pass: %v", err)
	}

	if err := GuardCommitmentActionSet(nil); err == nil {
		t.Fatalf("the empty action set must refuse")
	}
	five := append([]string(nil), CommitmentActions...)
	five = append(five, "prepare_order")
	if err := GuardCommitmentActionSet(five); err == nil {
		t.Fatalf("the five-word action set must refuse")
	}
	if err := GuardCommitmentActionSet([]string{"reserve_slot"}); err == nil {
		t.Fatalf("the non-dictionary action word must refuse")
	}
	if err := GuardCommitmentActionSet([]string{CommitmentActionPrepareOrder, CommitmentActionPrepareOrder}); err == nil {
		t.Fatalf("the duplicate action words must refuse")
	}
	if err := GuardCommitmentActionSet([]string{CommitmentActionPrepareOrder, CommitmentActionScheduleAppointment}); err != nil {
		t.Fatalf("the two-word dictionary action set must pass: %v", err)
	}

	if err := GuardCommitmentConsumptionLimit(0); err == nil {
		t.Fatalf("the zero consumption limit must refuse")
	}
	if err := GuardCommitmentConsumptionLimit(MaxCommitmentConsumption + 1); err == nil {
		t.Fatalf("the oversize consumption limit must refuse")
	}
	if err := GuardCommitmentConsumptionLimit(1); err != nil {
		t.Fatalf("the minimal consumption limit must pass: %v", err)
	}
}

func TestMaterialTermsJSONDeterministic(t *testing.T) {
	first, err := MaterialTermsJSON(validTerms())
	if err != nil {
		t.Fatalf("the valid material must serialize: %v", err)
	}
	second, err := MaterialTermsJSON(validTerms())
	if err != nil {
		t.Fatalf("the valid material must serialize again: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("the canonical serialization is deterministic: %q vs %q", first, second)
	}
	if bytes.Contains(first, []byte("  ")) {
		t.Fatalf("the canonical serialization carries no spaces: %q", first)
	}
	// The key order is the fixed struct order (scope first, consequences last).
	if !bytes.HasPrefix(first, []byte(`{"scope":`)) {
		t.Fatalf("the canonical serialization starts with the scope key: %q", first)
	}
	if !bytes.HasSuffix(first, []byte(`"consequences":"`+validTerms().Consequences+`"}`)) {
		t.Fatalf("the canonical serialization ends with the consequences key: %q", first)
	}
	changed := validTerms()
	changed.Price = CommitmentPrice{Kind: CommitmentPriceKindRange, MinCents: 400000, MaxCents: 600000}
	other, err := MaterialTermsJSON(changed)
	if err != nil {
		t.Fatalf("the changed material must serialize: %v", err)
	}
	if bytes.Equal(first, other) {
		t.Fatalf("the changed price must change the canonical bytes")
	}
	oversize := validTerms()
	oversize.Scope = strings.Repeat("д", MaxCommitmentConditionsByte)
	if _, err := MaterialTermsJSON(oversize); err == nil {
		t.Fatalf("the oversize canonical conditions must refuse")
	}
	priceJSON, err := PriceValueJSON(validTerms().Price)
	if err != nil {
		t.Fatalf("the price value must serialize: %v", err)
	}
	if !bytes.Contains(first, priceJSON) {
		t.Fatalf("the canonical price value is embedded byte-exact: %q vs %q", first, priceJSON)
	}
}

func TestGuardCreateCommitment(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	command := CreateCommitmentCommand{
		Status:           CommitmentStatusOffered,
		Terms:            validTerms(),
		ActionSet:        []string{CommitmentActionPrepareOrder},
		ValidUntil:       now.Add(24 * time.Hour),
		ConsumptionLimit: 1,
	}
	if err := GuardCreateCommitment(command, now); err != nil {
		t.Fatalf("the complete offered command must pass: %v", err)
	}
	draft := command
	draft.Status = CommitmentStatusDraft
	if err := GuardCreateCommitment(draft, now); err != nil {
		t.Fatalf("the draft command must pass: %v", err)
	}
	wrongStatus := command
	wrongStatus.Status = "approved"
	if err := GuardCreateCommitment(wrongStatus, now); err == nil {
		t.Fatalf("the non-dictionary status must refuse")
	}
	past := command
	past.ValidUntil = now.Add(-time.Hour)
	if err := GuardCreateCommitment(past, now); err == nil {
		t.Fatalf("the past validity must refuse")
	}
	unknownHole := command
	unknownHole.MaterialUnknowns = []string{CommitmentTermPrice}
	if err := GuardCreateCommitment(unknownHole, now); err != nil {
		t.Fatalf("the honest hole (unknown price) is a legal offered command: %v", err)
	}
	unknownForeign := command
	unknownForeign.MaterialUnknowns = []string{"brand"}
	if err := GuardCreateCommitment(unknownForeign, now); err == nil {
		t.Fatalf("the foreign unknown word must refuse")
	}
}

func TestDeriveCommitmentReadiness(t *testing.T) {
	base := CommitmentVersion{Status: CommitmentStatusOffered, VersionNo: 1}
	if got := DeriveCommitmentReadiness(base, 1); got != CommitmentReadinessConfirmable {
		t.Fatalf("the current offered version without holes is confirmable, got %q", got)
	}
	superseded := base
	superseded.VersionNo = 1
	if got := DeriveCommitmentReadiness(superseded, 2); got != CommitmentReadinessSuperseded {
		t.Fatalf("the older version is superseded, got %q", got)
	}
	draft := base
	draft.Status = CommitmentStatusDraft
	if got := DeriveCommitmentReadiness(draft, 1); got != CommitmentReadinessDraft {
		t.Fatalf("the draft status is the draft readiness, got %q", got)
	}
	hole := base
	hole.MaterialUnknowns = []string{CommitmentTermPrice}
	if got := DeriveCommitmentReadiness(hole, 1); got != CommitmentReadinessUnknownBlocked {
		t.Fatalf("the unknown mandatory term blocks the readiness, got %q", got)
	}
}

func TestDeriveConfirmationValidity(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	base := ConfirmationValidityInput{
		Now:               now,
		ExpiresAt:         now.Add(24 * time.Hour),
		MaxVersionNo:      1,
		ConfirmedVersion:  1,
		OwnerEpochPinned:  3,
		OwnerEpochCurrent: 3,
		LineagePinned:     "employee/1/2",
		LineageCurrent:    "employee/1/2",
	}
	if got := DeriveConfirmationValidity(base); got != ConfirmationValidityValid {
		t.Fatalf("the intact pins keep the confirmation valid, got %q", got)
	}
	expired := base
	expired.ExpiresAt = now.Add(-time.Minute)
	if got := DeriveConfirmationValidity(expired); got != ConfirmationValidityExpired {
		t.Fatalf("the passed expiry expires the confirmation, got %q", got)
	}
	superseded := base
	superseded.MaxVersionNo = 2
	if got := DeriveConfirmationValidity(superseded); got != ConfirmationValiditySuperseded {
		t.Fatalf("the newer version supersedes the confirmation, got %q", got)
	}
	ownerStale := base
	ownerStale.OwnerEpochCurrent = 4
	if got := DeriveConfirmationValidity(ownerStale); got != ConfirmationValidityOwnerStale {
		t.Fatalf("the moved owner epoch stales the confirmation, got %q", got)
	}
	releaseStale := base
	releaseStale.LineageCurrent = "employee/2/1"
	if got := DeriveConfirmationValidity(releaseStale); got != ConfirmationValidityReleaseStale {
		t.Fatalf("the moved release lineage stales the confirmation, got %q", got)
	}
	// The precedence follows the spec order: expired → superseded →
	// owner_stale → release_stale (the first honest word wins).
	both := base
	both.ExpiresAt = now.Add(-time.Minute)
	both.MaxVersionNo = 2
	both.OwnerEpochCurrent = 4
	if got := DeriveConfirmationValidity(both); got != ConfirmationValidityExpired {
		t.Fatalf("the expiry word takes the precedence, got %q", got)
	}
}

func TestCommitmentTokenBindsExactVersion(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	material := []byte(`{"scope":"Замена масла"}`)
	token := CommitmentToken(key, commitmentTestID(), 1, material)
	if len(token) != 64 {
		t.Fatalf("the token is the hex char(64), got %d", len(token))
	}
	if token != CommitmentToken(key, commitmentTestID(), 1, material) {
		t.Fatalf("the token is deterministic under the same key/version/material")
	}
	if token == CommitmentToken(key, commitmentTestID(), 2, material) {
		t.Fatalf("the token binds the exact version")
	}
	otherMaterial := append([]byte(nil), material...)
	otherMaterial[len(otherMaterial)-2] = 'X'
	if token == CommitmentToken(key, commitmentTestID(), 1, otherMaterial) {
		t.Fatalf("the token binds the exact material bytes")
	}
	if token == CommitmentToken([]byte("fedcba9876543210fedcba9876543210"), commitmentTestID(), 1, material) {
		t.Fatalf("the token binds the key")
	}
	// The domain separation: the identifier token of the same key and the
	// same material shape never collides with the commitment token.
	if token == IdentifierToken(key, "commitment", string(material)) {
		t.Fatalf("the domain separation prefix keeps the token families apart")
	}
}

func TestNewCommitmentKey(t *testing.T) {
	first, err := NewCommitmentKey()
	if err != nil {
		t.Fatalf("the key must generate: %v", err)
	}
	second, err := NewCommitmentKey()
	if err != nil {
		t.Fatalf("the key must generate again: %v", err)
	}
	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("the commitment key is 32 bytes, got %d and %d", len(first), len(second))
	}
	if bytes.Equal(first, second) {
		t.Fatalf("the keys never repeat")
	}
}

func TestGuardConfirmEvidence(t *testing.T) {
	payload := []byte("Да, согласен на замену масла за 4500 рублей, приезжайте завтра к десяти.")
	material := ConfirmCommitmentMaterial{
		SubjectKind: "client",
		SubjectRef:  "case-client",
		TurnPayload: payload,
		Spans:       []string{"Да, согласен на замену масла за 4500 рублей"},
	}
	if err := GuardConfirmEvidence(material); err != nil {
		t.Fatalf("the verbatim span of the payload must pass: %v", err)
	}
	noSubject := material
	noSubject.SubjectRef = ""
	if err := GuardConfirmEvidence(noSubject); err == nil {
		t.Fatalf("the subjectless evidence must refuse")
	}
	noSpans := material
	noSpans.Spans = nil
	if err := GuardConfirmEvidence(noSpans); err == nil {
		t.Fatalf("the spanless evidence must refuse")
	}
	invented := material
	invented.Spans = []string{"Клиент согласен на всё"}
	if err := GuardConfirmEvidence(invented); err == nil {
		t.Fatalf("the invented consent must refuse — the span is not in the payload")
	}
	dup := material
	dup.Spans = []string{"Да, согласен на замену масла", "Да, согласен на замену масла"}
	if err := GuardConfirmEvidence(dup); err == nil {
		t.Fatalf("the duplicate spans must refuse")
	}
	oversize := material
	oversize.Spans = []string{strings.Repeat("а", MaxObjectionSpanRunes+1)}
	if err := GuardConfirmEvidence(oversize); err == nil {
		t.Fatalf("the oversize span must refuse")
	}
	nine := material
	nine.Spans = []string{}
	for i := 0; i <= MaxObjectionSpans; i++ {
		nine.Spans = append(nine.Spans, fmt.Sprintf("слово%d", i))
	}
	if err := GuardConfirmEvidence(nine); err == nil {
		t.Fatalf("the nine-span evidence must refuse")
	}
}

func TestGuardConfirmReadinessBranches(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	validUntil := now.Add(24 * time.Hour)
	if err := GuardConfirmReadiness(CommitmentReadinessConfirmable, validUntil, now); err != nil {
		t.Fatalf("the confirmable readiness with the future validity must pass: %v", err)
	}
	if err := GuardConfirmReadiness(CommitmentReadinessDraft, validUntil, now); err == nil {
		t.Fatalf("the draft readiness must refuse")
	}
	if err := GuardConfirmReadiness(CommitmentReadinessSuperseded, validUntil, now); err == nil {
		t.Fatalf("the superseded readiness must refuse")
	}
	if err := GuardConfirmReadiness(CommitmentReadinessUnknownBlocked, validUntil, now); err == nil {
		t.Fatalf("the unknown-blocked readiness must refuse")
	}
	if err := GuardConfirmReadiness(CommitmentReadinessConfirmable, now.Add(-time.Minute), now); err == nil {
		t.Fatalf("the passed validity must refuse")
	}
}

func TestGuardDeclineCommitment(t *testing.T) {
	payload := []byte("Нет, дороговато — давайте без замены фильтра.")
	material := ConfirmCommitmentMaterial{
		SubjectKind: "client",
		SubjectRef:  "case-client",
		TurnPayload: payload,
		Spans:       []string{"Нет, дороговато"},
	}
	if err := GuardDeclineCommitment(CommitmentDeclineOutcomeDeclined, material); err != nil {
		t.Fatalf("the honest decline must pass: %v", err)
	}
	if err := GuardDeclineCommitment(CommitmentDeclineOutcomeChangeRequested, material); err != nil {
		t.Fatalf("the change request must pass: %v", err)
	}
	if err := GuardDeclineCommitment("postponed", material); err == nil {
		t.Fatalf("the non-dictionary outcome must refuse")
	}
	invented := material
	invented.Spans = []string{"Клиент отказался"}
	if err := GuardDeclineCommitment(CommitmentDeclineOutcomeDeclined, invented); err == nil {
		t.Fatalf("the invented decline words must refuse")
	}
}

func TestGuardPresentedCommitmentPair(t *testing.T) {
	if err := GuardPresentedCommitment(PresentedCommitment{}); err != nil {
		t.Fatalf("the empty pair must pass: %v", err)
	}
	if err := GuardPresentedCommitment(PresentedCommitment{CommitmentID: commitmentTestID(), VersionNo: 1}); err != nil {
		t.Fatalf("the full pair must pass: %v", err)
	}
	if err := GuardPresentedCommitment(PresentedCommitment{CommitmentID: commitmentTestID()}); err == nil {
		t.Fatalf("the half pair must refuse")
	}
	if err := GuardPresentedCommitment(PresentedCommitment{VersionNo: 1}); err == nil {
		t.Fatalf("the half pair must refuse")
	}
}

func TestCurrentCommitmentVersion(t *testing.T) {
	versions := []CommitmentVersion{
		{VersionNo: 1, Status: CommitmentStatusOffered},
		{VersionNo: 3, Status: CommitmentStatusOffered},
		{VersionNo: 2, Status: CommitmentStatusDraft},
	}
	current := CurrentCommitmentVersion(versions)
	if current == nil || current.VersionNo != 3 {
		t.Fatalf("the current version is the maximal number, got %+v", current)
	}
	if CurrentCommitmentVersion(nil) != nil {
		t.Fatalf("the empty history carries no current version")
	}
}

func TestConfirmedLadderIsAdditive(t *testing.T) {
	// The confirmed word joins the dictionary once.
	if !inDictionary(WorkItemStates, WorkItemStateConfirmed) {
		t.Fatalf("the confirmed word joins the work item dictionary")
	}
	// The new transitions: in_work → confirmed and waiting → confirmed.
	if !TransitionAllowed(WorkItemStateInWork, WorkItemStateConfirmed) {
		t.Fatalf("in_work → confirmed is the machine step of the confirmation")
	}
	if !TransitionAllowed(WorkItemStateWaiting, WorkItemStateConfirmed) {
		t.Fatalf("waiting → confirmed is the machine step of the confirmation")
	}
	// The exits of confirmed: the operator completions only.
	if !TransitionAllowed(WorkItemStateConfirmed, WorkItemStateResolved) {
		t.Fatalf("confirmed → resolved stays allowed")
	}
	if !TransitionAllowed(WorkItemStateConfirmed, WorkItemStateCancelled) {
		t.Fatalf("confirmed → cancelled stays allowed")
	}
	// No re-open and no self-loop of the confirmed state.
	if TransitionAllowed(WorkItemStateConfirmed, WorkItemStateInWork) {
		t.Fatalf("confirmed never re-opens to in_work")
	}
	if TransitionAllowed(WorkItemStateConfirmed, WorkItemStateWaiting) {
		t.Fatalf("confirmed never re-opens to waiting")
	}
	if TransitionAllowed(WorkItemStateConfirmed, WorkItemStateConfirmed) {
		t.Fatalf("confirmed never loops to itself")
	}
	if TransitionAllowed(WorkItemStateConfirmed, WorkItemStateSplit) {
		t.Fatalf("confirmed never splits in this slice")
	}
	// The CONTROL BRANCH: every pre-existing transition stays byte-identical.
	if !TransitionAllowed(WorkItemStateInWork, WorkItemStateWaiting) || !TransitionAllowed(WorkItemStateInWork, WorkItemStateResolved) || !TransitionAllowed(WorkItemStateInWork, WorkItemStateCancelled) {
		t.Fatalf("the in_work exits of 2.2 are untouched")
	}
	if !TransitionAllowed(WorkItemStateWaiting, WorkItemStateInWork) || !TransitionAllowed(WorkItemStateWaiting, WorkItemStateResolved) || !TransitionAllowed(WorkItemStateWaiting, WorkItemStateCancelled) {
		t.Fatalf("the waiting exits of 2.2 are untouched")
	}
	// The confirmed state is NOT terminal (the completions still run).
	if TerminalWorkItemStates[WorkItemStateConfirmed] {
		t.Fatalf("confirmed is a live state — the completions still run")
	}
	// The untriaged discipline of 2.2 is untouched.
	if TransitionAllowed(WorkItemStateUntriaged, WorkItemStateConfirmed) {
		t.Fatalf("untriaged is classified, never transitioned")
	}
	// The terminal states stay terminal.
	for _, terminal := range []string{WorkItemStateResolved, WorkItemStateSplit, WorkItemStateMerged} {
		if TransitionAllowed(terminal, WorkItemStateConfirmed) {
			t.Fatalf("the terminal state %q never transitions", terminal)
		}
	}
	// The honest next action of the confirmed state exists.
	if RecommendedNextAction(WorkItemStateConfirmed) == "" {
		t.Fatalf("the confirmed state carries the honest next action")
	}
	// The classification ladder of 2.2 is untouched (untriaged/in_work/waiting).
	if !ClassifyAllowedFrom(WorkItemStateUntriaged) || !ClassifyAllowedFrom(WorkItemStateInWork) || !ClassifyAllowedFrom(WorkItemStateWaiting) {
		t.Fatalf("the classification ladder of 2.2 is untouched")
	}
	// A confirmed work item is not re-classified by the transition ladder
	// (the confirm step is the machine's own — the honest boundary).
	if ClassifyAllowedFrom(WorkItemStateConfirmed) {
		t.Fatalf("the confirmed state joins no classification ladder")
	}
}

func TestCommitmentOutboxPayloadsAreSafe(t *testing.T) {
	// The version event carries the identifiers and the words only — the
	// material conditions never enter the envelope.
	versionJSON, err := CommitmentVersionCreatedPayloadJSON(CommitmentVersionCreatedPayload{
		TenantID: commitmentTestID(), CaseID: commitmentTestID(), WorkItemID: commitmentTestID(),
		CommitmentID: commitmentTestID(), VersionNo: 1, Status: CommitmentStatusOffered,
		ActorID: commitmentTestID(), OccurredAt: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("the version payload must serialize: %v", err)
	}
	if bytes.Contains(versionJSON, []byte("scope")) || bytes.Contains(versionJSON, []byte("price")) {
		t.Fatalf("the material conditions never enter the outbox envelope: %q", versionJSON)
	}
	confirmedJSON, err := CommitmentConfirmedPayloadJSON(CommitmentConfirmedPayload{
		TenantID: commitmentTestID(), CaseID: commitmentTestID(), WorkItemID: commitmentTestID(),
		CommitmentID: commitmentTestID(), VersionNo: 1, ConfirmationID: commitmentTestID(),
		SubjectKind: "client", SubjectRef: "case-client",
		ActorID: commitmentTestID(), OccurredAt: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("the confirmation payload must serialize: %v", err)
	}
	if bytes.Contains(confirmedJSON, []byte("span")) {
		t.Fatalf("the span excerpts never enter the outbox envelope: %q", confirmedJSON)
	}
	declinedJSON, err := CommitmentDeclinedPayloadJSON(CommitmentDeclinedPayload{
		TenantID: commitmentTestID(), CaseID: commitmentTestID(), WorkItemID: commitmentTestID(),
		CommitmentID: commitmentTestID(), VersionNo: 1, Outcome: CommitmentDeclineOutcomeDeclined,
		ActorID: commitmentTestID(), OccurredAt: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("the decline payload must serialize: %v", err)
	}
	if bytes.Contains(declinedJSON, []byte("span")) {
		t.Fatalf("the span excerpts never enter the decline envelope: %q", declinedJSON)
	}
	if DedupKeyCommitmentVersionCreated(commitmentTestID(), 1) == DedupKeyCommitmentVersionCreated(commitmentTestID(), 2) {
		t.Fatalf("the dedup key binds the exact version")
	}
}
