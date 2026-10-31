package domain

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 3.3 unit matrix: the dictionaries of the epic (the purposes,
// the recipient kinds, the ack requirements, the seven delivery states and
// the seven dispatch kinds, the relay observations, the stale reasons, the
// adapter outcomes and the readiness words), the envelope create guard
// (the 2.9 pin discipline for the offer, the all-or-nothing triple), the
// canonical bytes and the content binding token (the separated token
// space), the readiness derivation with its ordered stale fences, the
// seven-state read-time derivation over the append-only events, the relay
// and outcome guards, the safe outbox perimeter (the material never leaves
// the row), the idempotency key of the version pin and the deterministic
// outbound payload.

// extTestTenant builds the deterministic tenant id of the local matrix.
func extTestTenant() id.UUID { return id.MustParse("019a2f4c-2900-7000-8000-0000000000aa") }

// extTestCase builds the deterministic case id.
func extTestCase() id.UUID { return id.MustParse("019a2f4c-2900-7000-8000-0000000000bb") }

// extTestWorkItem builds the deterministic work item id.
func extTestWorkItem() id.UUID { return id.MustParse("019a2f4c-2900-7000-8000-0000000000cc") }

// extTestID builds the deterministic stable external id.
func extTestID() id.UUID { return id.MustParse("019a2f4c-2900-7000-8000-0000000000ee") }

// extTestOwnerRef builds the deterministic owner ref.
func extTestOwnerRef() id.UUID { return id.MustParse("019a2f4c-2900-7000-8000-0000000000dd") }

// extTestPinnedCommitment builds the deterministic pinned 2.9 id.
func extTestPinnedCommitment() id.UUID { return id.MustParse("019a2f4c-2900-7000-8000-0000000000f1") }

// extTestPin builds the complete valid pin triple.
func extTestPin() ExternalCommitmentPin {
	return ExternalCommitmentPin{
		CommitmentID: extTestPinnedCommitment(),
		VersionNo:    4,
		ContentToken: strings.Repeat("ab", 32),
	}
}

// extTestNow is the fixed clock of the local matrix.
func extTestNow() time.Time { return time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC) }

// extTestCommand builds the complete valid create command: a promise
// without the pin (the honest default) and the future validity.
func extTestCommand() CreateExternalCommitmentCommand {
	return CreateExternalCommitmentCommand{
		TenantID:         extTestTenant(),
		CaseID:           extTestCase(),
		WorkItemID:       extTestWorkItem(),
		ExtCommitmentID:  extTestID(),
		VersionNo:        1,
		Purpose:          ExternalPurposePromise,
		RecipientKind:    ExternalRecipientClient,
		Channel:          "phone",
		ContentText:      "Обещаем перезвонить завтра к 10:00 по вопросу замены масла.",
		ValidUntil:       extTestNow().Add(24 * time.Hour),
		ExpectedNextStep: "Оператор перезванивает и подтверждает слот",
		AckRequirement:   ExternalAckRequired,
		OwnerKind:        OwnerKindHuman,
		OwnerRef:         extTestOwnerRef(),
		OwnerEpoch:       2,
		DecisionNo:       7,
		ProfileVersion:   3,
		PolicyOutcomeNo:  5,
	}
}

// extTestVersion builds the complete valid envelope version row.
func extTestVersion() ExternalCommitment {
	command := extTestCommand()
	return ExternalCommitment{
		TenantID:         command.TenantID,
		CaseID:           command.CaseID,
		WorkItemID:       command.WorkItemID,
		ExtCommitmentID:  command.ExtCommitmentID,
		VersionNo:        command.VersionNo,
		Purpose:          command.Purpose,
		RecipientKind:    command.RecipientKind,
		Channel:          command.Channel,
		ContentText:      command.ContentText,
		ValidUntil:       command.ValidUntil,
		ExpectedNextStep: command.ExpectedNextStep,
		AckRequirement:   command.AckRequirement,
		Pin:              command.Pin,
		OwnerKind:        command.OwnerKind,
		OwnerRef:         command.OwnerRef,
		OwnerEpoch:       command.OwnerEpoch,
		DecisionNo:       command.DecisionNo,
		ProfileVersion:   command.ProfileVersion,
		PolicyOutcomeNo:  command.PolicyOutcomeNo,
		ContentToken:     strings.Repeat("cd", 32),
		ActorID:          extTestOwnerRef(),
		CreatedAt:        extTestNow(),
	}
}

// extTestEvent builds one dispatch event of the given number and kind.
func extTestEvent(eventNo int64, kind string) ExternalDispatchEvent {
	return ExternalDispatchEvent{
		TenantID:              extTestTenant(),
		CaseID:                extTestCase(),
		ExtCommitmentID:       extTestID(),
		VersionNo:             1,
		EventID:               id.MustParse("019a2f4c-2900-7000-8000-0000000000e1"),
		EventNo:               eventNo,
		Kind:                  kind,
		ActorID:               extTestOwnerRef(),
		CreatedAt:             extTestNow().Add(time.Duration(eventNo) * time.Minute),
		ExternalCorrelationID: "corr-" + string(rune('a'+eventNo-1)) + "-123",
	}
}

// ---------------------------------------------------------------------------
// The dictionaries (every word by name)
// ---------------------------------------------------------------------------

func TestExternalPurposeDictionaryCarriesTheEpicWords(t *testing.T) {
	if len(ExternalPurposeWords) != 4 {
		t.Fatalf("the purpose dictionary carries four words, got %v", ExternalPurposeWords)
	}
	want := []string{ExternalPurposePromise, ExternalPurposeOffer, ExternalPurposeCallback, ExternalPurposeDeadlineUpdate}
	for i, word := range want {
		if ExternalPurposeWords[i] != word {
			t.Fatalf("the purpose word %d must be %q, got %q", i, word, ExternalPurposeWords[i])
		}
		if !inDictionary(ExternalPurposeWords, word) {
			t.Fatalf("the purpose word %q must be in its own dictionary", word)
		}
	}
}

func TestExternalRecipientKindsNeverIncludeTheExecutor(t *testing.T) {
	if len(ExternalRecipientKindWords) != 2 {
		t.Fatalf("the recipient dictionary carries two words, got %v", ExternalRecipientKindWords)
	}
	if ExternalRecipientKindWords[0] != ExternalRecipientClient || ExternalRecipientKindWords[1] != ExternalRecipientOperator {
		t.Fatalf("the recipient dictionary carries client and operator, got %v", ExternalRecipientKindWords)
	}
	for _, word := range ExternalRecipientKindWords {
		if word == "executor" {
			t.Fatalf("the executor recipient honestly never exists in this slice (Epic 5)")
		}
	}
}

func TestExternalAckRequirementDictionary(t *testing.T) {
	if len(ExternalAckRequirementWords) != 2 {
		t.Fatalf("the ack dictionary carries two words, got %v", ExternalAckRequirementWords)
	}
	if ExternalAckRequirementWords[0] != ExternalAckRequired || ExternalAckRequirementWords[1] != ExternalAckNotRequired {
		t.Fatalf("the ack dictionary carries required and not_required, got %v", ExternalAckRequirementWords)
	}
}

func TestExternalDeliveryStatesAreTheSevenWordsOfAC7(t *testing.T) {
	want := []string{
		"persisted", "queued", "submitted", "delivered",
		"acknowledged", "failed", "unknown",
	}
	if len(ExternalDeliveryStateWords) != 7 {
		t.Fatalf("the delivery dictionary carries seven words, got %v", ExternalDeliveryStateWords)
	}
	for i, word := range want {
		if ExternalDeliveryStateWords[i] != word {
			t.Fatalf("the delivery word %d must be %q, got %q", i, word, ExternalDeliveryStateWords[i])
		}
	}
}

func TestExternalDispatchKindsAreSevenWordsWithoutQueued(t *testing.T) {
	if len(ExternalDispatchKindWords) != 7 {
		t.Fatalf("the dispatch kind dictionary carries seven kinds, got %v", ExternalDispatchKindWords)
	}
	for _, kind := range ExternalDispatchKindWords {
		if kind == "queued" {
			t.Fatalf("queued is the outbox record itself, never a dispatch event kind (the 2.12 dispatched precedent)")
		}
	}
	for _, kind := range []string{
		ExternalDispatchAttemptSubmitted, ExternalDispatchAttemptFailed,
		ExternalDispatchAttemptUnknown, ExternalDispatchReceiptDelivered,
		ExternalDispatchReceiptAcknowledged, ExternalDispatchReceiptFailed,
		ExternalDispatchCancelledStale,
	} {
		if !inDictionary(ExternalDispatchKindWords, kind) {
			t.Fatalf("the dispatch kind %q must be in its own dictionary", kind)
		}
	}
}

func TestRelayObservationDictionaryCarriesTheThreeObservations(t *testing.T) {
	if len(RelayObservationWords) != 3 {
		t.Fatalf("the relay dictionary carries three words, got %v", RelayObservationWords)
	}
	for _, word := range []string{RelayObservationDelivered, RelayObservationAcknowledged, RelayObservationFailed} {
		if !inDictionary(RelayObservationWords, word) {
			t.Fatalf("the relay word %q must be in its own dictionary", word)
		}
	}
}

func TestExternalStaleReasonDictionaryCarriesTheFourFences(t *testing.T) {
	if len(ExternalStaleReasonWords) != 4 {
		t.Fatalf("the stale reason dictionary carries four words, got %v", ExternalStaleReasonWords)
	}
	for _, word := range []string{
		ExternalStaleNewerVersion, ExternalStaleExpired,
		ExternalStaleOwnerEpoch, ExternalStaleControlMode,
	} {
		if !inDictionary(ExternalStaleReasonWords, word) {
			t.Fatalf("the stale reason %q must be in its own dictionary", word)
		}
	}
}

func TestDispatchOutcomeDictionaryCarriesTheAdapterContract(t *testing.T) {
	if len(DispatchOutcomeWords) != 4 {
		t.Fatalf("the outcome dictionary carries four words, got %v", DispatchOutcomeWords)
	}
	for _, word := range []string{
		DispatchOutcomeAccepted, DispatchOutcomeDelivered,
		DispatchOutcomeFailed, DispatchOutcomeTimeout,
	} {
		if !inDictionary(DispatchOutcomeWords, word) {
			t.Fatalf("the outcome word %q must be in its own dictionary", word)
		}
	}
}

func TestExternalReadinessDictionaryCarriesTheFiveWords(t *testing.T) {
	if len(ExternalReadinessWords) != 5 {
		t.Fatalf("the readiness dictionary carries five words, got %v", ExternalReadinessWords)
	}
	for _, word := range []string{
		ExternalReadinessReady, ExternalReadinessSuperseded,
		ExternalReadinessExpired, ExternalReadinessOwnerStale,
		ExternalReadinessPinBlocked,
	} {
		if !inDictionary(ExternalReadinessWords, word) {
			t.Fatalf("the readiness word %q must be in its own dictionary", word)
		}
	}
}

func TestMaxDispatchAttemptsBoundIsThree(t *testing.T) {
	if MaxDispatchAttempts != 3 {
		t.Fatalf("the bounded retry limit is three attempts, got %d", MaxDispatchAttempts)
	}
}

// ---------------------------------------------------------------------------
// The create guard (the dictionary honesty, the pin discipline)
// ---------------------------------------------------------------------------

func TestGuardCreateAcceptsTheCompleteValidCommand(t *testing.T) {
	if err := GuardCreateExternalCommitment(extTestCommand(), extTestNow()); err != nil {
		t.Fatalf("the complete valid command must pass, got %v", err)
	}
}

func TestGuardCreateAcceptsEveryNonOfferPurposeWithoutAPin(t *testing.T) {
	for _, purpose := range []string{ExternalPurposePromise, ExternalPurposeCallback, ExternalPurposeDeadlineUpdate} {
		command := extTestCommand()
		command.Purpose = purpose
		if err := GuardCreateExternalCommitment(command, extTestNow()); err != nil {
			t.Fatalf("the purpose %q creates honestly without the pin, got %v", purpose, err)
		}
	}
}

func TestGuardCreateOfferDemandsTheExactVersionPin(t *testing.T) {
	command := extTestCommand()
	command.Purpose = ExternalPurposeOffer
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the offer without the pin is the dictionary refusal of 2.9 — got nil")
	}
	command.Pin = extTestPin()
	if err := GuardCreateExternalCommitment(command, extTestNow()); err != nil {
		t.Fatalf("the offer with the complete pin triple passes, got %v", err)
	}
}

func TestGuardCreateRefusesTheForeignPurposeWord(t *testing.T) {
	command := extTestCommand()
	command.Purpose = "gesture"
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the foreign purpose word is impossible, got nil")
	}
}

func TestGuardCreateRefusesTheForeignRecipientKind(t *testing.T) {
	command := extTestCommand()
	command.RecipientKind = "executor"
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the executor recipient honestly never exists in this slice, got nil")
	}
}

func TestGuardCreateAcceptsEveryChannelOfTheNineWordMirror(t *testing.T) {
	for _, channel := range ChannelOrder {
		command := extTestCommand()
		command.Channel = channel
		if err := GuardCreateExternalCommitment(command, extTestNow()); err != nil {
			t.Fatalf("the channel %q belongs to the nine-word mirror, got %v", channel, err)
		}
	}
	command := extTestCommand()
	command.Channel = "pigeon_post"
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the foreign channel word is impossible, got nil")
	}
}

func TestGuardCreateContentBounds(t *testing.T) {
	command := extTestCommand()
	command.ContentText = "   "
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the empty content is impossible, got nil")
	}
	command.ContentText = strings.Repeat("с", MaxExternalCommitmentContentRunes)
	if err := GuardCreateExternalCommitment(command, extTestNow()); err != nil {
		t.Fatalf("the content of the exact bound passes, got %v", err)
	}
	command.ContentText = strings.Repeat("с", MaxExternalCommitmentContentRunes+1)
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the content above the bound is impossible, got nil")
	}
}

func TestGuardCreateValidUntilMustLieInTheFuture(t *testing.T) {
	now := extTestNow()
	command := extTestCommand()
	command.ValidUntil = now.Add(-time.Hour)
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the expired window is impossible at creation, got nil")
	}
	command.ValidUntil = now
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the boundary instant is not the future — the fence fails closed, got nil")
	}
	command.ValidUntil = now.Add(time.Minute)
	if err := GuardCreateExternalCommitment(command, now); err != nil {
		t.Fatalf("the future window passes, got %v", err)
	}
}

func TestGuardCreateExpectedNextStepBounds(t *testing.T) {
	command := extTestCommand()
	command.ExpectedNextStep = " "
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the empty next step is impossible, got nil")
	}
	command.ExpectedNextStep = strings.Repeat("ш", MaxExpectedNextStepRunes)
	if err := GuardCreateExternalCommitment(command, extTestNow()); err != nil {
		t.Fatalf("the next step of the exact bound passes, got %v", err)
	}
	command.ExpectedNextStep = strings.Repeat("ш", MaxExpectedNextStepRunes+1)
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the next step above the bound is impossible, got nil")
	}
}

func TestGuardCreateRefusesTheForeignAckWord(t *testing.T) {
	command := extTestCommand()
	command.AckRequirement = "maybe"
	if err := GuardCreateExternalCommitment(command, extTestNow()); err == nil {
		t.Fatalf("the foreign ack word is impossible, got nil")
	}
}

func TestGuardCreateRefusesTheInvalidIdentifiers(t *testing.T) {
	now := extTestNow()
	command := extTestCommand()
	command.TenantID = "not-a-uuid"
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the invalid tenant is impossible, got nil")
	}
	command = extTestCommand()
	command.CaseID = ""
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the empty case is impossible, got nil")
	}
	command = extTestCommand()
	command.WorkItemID = "019a2f4c-2900-7000-8000-0000000000zz"
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the malformed work item is impossible, got nil")
	}
	command = extTestCommand()
	command.VersionNo = 0
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the zero version number is impossible (the rows start at one), got nil")
	}
}

func TestGuardCreateOwnerSnapshotDiscipline(t *testing.T) {
	now := extTestNow()
	command := extTestCommand()
	command.OwnerKind = "department"
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("a department is never an owner kind, got nil")
	}
	command = extTestCommand()
	command.OwnerRef = "019a2f4c-2900-7000-8000-0000000000q2"
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the malformed owner ref is impossible, got nil")
	}
	command = extTestCommand()
	command.OwnerEpoch = 0
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the dead owner epoch is impossible (the snapshot pins the LIVE epoch), got nil")
	}
}

func TestGuardCreateLineageNumbersAreHonest(t *testing.T) {
	now := extTestNow()
	command := extTestCommand()
	command.DecisionNo, command.ProfileVersion, command.PolicyOutcomeNo = 0, 0, 0
	if err := GuardCreateExternalCommitment(command, now); err != nil {
		t.Fatalf("the zero lineage is the honest none, got %v", err)
	}
	command.DecisionNo = -1
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the negative decision number is impossible, got nil")
	}
	command = extTestCommand()
	command.ProfileVersion = -2
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the negative profile version is impossible, got nil")
	}
	command = extTestCommand()
	command.PolicyOutcomeNo = -3
	if err := GuardCreateExternalCommitment(command, now); err == nil {
		t.Fatalf("the negative policy outcome is impossible, got nil")
	}
}

func TestGuardCreatePinIsAnAllOrNothingTriple(t *testing.T) {
	now := extTestNow()
	partial := []ExternalCommitmentPin{
		{CommitmentID: extTestPinnedCommitment()},
		{VersionNo: 4},
		{ContentToken: strings.Repeat("ab", 32)},
		{CommitmentID: extTestPinnedCommitment(), VersionNo: 4},
		{CommitmentID: extTestPinnedCommitment(), ContentToken: strings.Repeat("ab", 32)},
		{VersionNo: 4, ContentToken: strings.Repeat("ab", 32)},
	}
	for i, pin := range partial {
		command := extTestCommand()
		command.Purpose = ExternalPurposeCallback
		command.Pin = pin
		if err := GuardCreateExternalCommitment(command, now); err == nil {
			t.Fatalf("the partial pin %d is the all-or-nothing refusal, got nil", i)
		}
	}
	command := extTestCommand()
	command.Purpose = ExternalPurposeCallback
	command.Pin = extTestPin()
	if err := GuardCreateExternalCommitment(command, now); err != nil {
		t.Fatalf("the complete pin triple is optional but valid for the non-offer purpose, got %v", err)
	}
}

func TestGuardExternalCommitmentPinTokenShape(t *testing.T) {
	for i, token := range []string{
		"", strings.Repeat("a", 63), strings.Repeat("a", 65),
		strings.Repeat("AB", 32), strings.Repeat("zz", 32),
	} {
		pin := ExternalCommitmentPin{CommitmentID: extTestPinnedCommitment(), VersionNo: 1, ContentToken: token}
		if err := GuardExternalCommitmentPin(pin); err == nil {
			t.Fatalf("the token shape %d is impossible, got nil", i)
		}
	}
	pin := ExternalCommitmentPin{CommitmentID: extTestPinnedCommitment(), VersionNo: 1, ContentToken: strings.Repeat("0f", 32)}
	if err := GuardExternalCommitmentPin(pin); err != nil {
		t.Fatalf("the honest 64-hex token passes, got %v", err)
	}
	if pin.Present() != true {
		t.Fatalf("the carried pin is present")
	}
	if (ExternalCommitmentPin{}).Present() {
		t.Fatalf("the zero pin is not present")
	}
}

// ---------------------------------------------------------------------------
// The canonical bytes and the content binding token
// ---------------------------------------------------------------------------

func TestExternalCommitmentCanonicalBytesCoverEveryMaterialField(t *testing.T) {
	base := extTestVersion()
	original := ExternalCommitmentCanonicalBytes(base)
	if len(original) == 0 {
		t.Fatalf("the canonical bytes are never empty")
	}
	mutations := map[string]func(*ExternalCommitment){
		"tenant":            func(v *ExternalCommitment) { v.TenantID = id.MustParse("019a2f4c-2900-7000-8000-0000000000ab") },
		"case":              func(v *ExternalCommitment) { v.CaseID = id.MustParse("019a2f4c-2900-7000-8000-0000000000bc") },
		"work item":         func(v *ExternalCommitment) { v.WorkItemID = id.MustParse("019a2f4c-2900-7000-8000-0000000000cd") },
		"ext id":            func(v *ExternalCommitment) { v.ExtCommitmentID = id.MustParse("019a2f4c-2900-7000-8000-0000000000ef") },
		"version":           func(v *ExternalCommitment) { v.VersionNo = 2 },
		"purpose":           func(v *ExternalCommitment) { v.Purpose = ExternalPurposeOffer },
		"recipient":         func(v *ExternalCommitment) { v.RecipientKind = ExternalRecipientOperator },
		"channel":           func(v *ExternalCommitment) { v.Channel = "email" },
		"content":           func(v *ExternalCommitment) { v.ContentText = "Изменённый текст конверта" },
		"valid until":       func(v *ExternalCommitment) { v.ValidUntil = v.ValidUntil.Add(time.Hour) },
		"next step":         func(v *ExternalCommitment) { v.ExpectedNextStep = "Другой следующий шаг" },
		"ack":               func(v *ExternalCommitment) { v.AckRequirement = ExternalAckNotRequired },
		"pin commitment":    func(v *ExternalCommitment) { v.Pin.CommitmentID = id.MustParse("019a2f4c-2900-7000-8000-0000000000f2") },
		"pin version":       func(v *ExternalCommitment) { v.Pin.VersionNo = 5 },
		"pin token":         func(v *ExternalCommitment) { v.Pin.ContentToken = strings.Repeat("ef", 32) },
		"owner kind":        func(v *ExternalCommitment) { v.OwnerKind = OwnerKindStaffy },
		"owner ref":         func(v *ExternalCommitment) { v.OwnerRef = id.MustParse("019a2f4c-2900-7000-8000-0000000000de") },
		"owner epoch":       func(v *ExternalCommitment) { v.OwnerEpoch = 3 },
		"decision no":       func(v *ExternalCommitment) { v.DecisionNo = 8 },
		"profile version":   func(v *ExternalCommitment) { v.ProfileVersion = 4 },
		"policy outcome no": func(v *ExternalCommitment) { v.PolicyOutcomeNo = 6 },
	}
	for name, mutate := range mutations {
		mutated := extTestVersion()
		mutate(&mutated)
		if bytes.Equal(ExternalCommitmentCanonicalBytes(mutated), original) {
			t.Fatalf("the material field %q must participate in the canonical bytes", name)
		}
	}
	again := extTestVersion()
	if !bytes.Equal(ExternalCommitmentCanonicalBytes(again), original) {
		t.Fatalf("the canonical bytes of the equal material are byte-equal (the determinism)")
	}
}

func TestExternalCommitmentTokenSpaceIsSeparatedFrom29(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	canonical := ExternalCommitmentCanonicalBytes(extTestVersion())
	version := extTestVersion()
	external := ExternalCommitmentToken(key, version.ExtCommitmentID, version.VersionNo, canonical)
	internal := CommitmentToken(key, id.UUID(version.ExtCommitmentID), version.VersionNo, canonical)
	if external == internal {
		t.Fatalf("the external token space is separated from the 2.9 space by the prefix")
	}
	if external != ExternalCommitmentContentToken(key, version) {
		t.Fatalf("the row re-computation is the drain re-check basis (the token of the carried row)")
	}
}

func TestExternalCommitmentTokenBindsTheExactVersion(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	version := extTestVersion()
	canonical := ExternalCommitmentCanonicalBytes(version)
	token := ExternalCommitmentToken(key, version.ExtCommitmentID, version.VersionNo, canonical)
	if len(token) != 64 || !isHex64(token) {
		t.Fatalf("the token is the 64-hex HMAC print, got %q", token)
	}
	if ExternalCommitmentToken(key, version.ExtCommitmentID, version.VersionNo, canonical) != token {
		t.Fatalf("the token of the same material is deterministic")
	}
	other := extTestVersion()
	other.VersionNo = 2
	if ExternalCommitmentToken(key, other.ExtCommitmentID, other.VersionNo, ExternalCommitmentCanonicalBytes(other)) == token {
		t.Fatalf("a changed version never matches the old token")
	}
	other = extTestVersion()
	other.ExtCommitmentID = id.MustParse("019a2f4c-2900-7000-8000-0000000000ef")
	if ExternalCommitmentToken(key, other.ExtCommitmentID, other.VersionNo, ExternalCommitmentCanonicalBytes(other)) == token {
		t.Fatalf("another envelope never matches the token")
	}
	otherKey := []byte(strings.Repeat("j", 32))
	if ExternalCommitmentToken(otherKey, version.ExtCommitmentID, version.VersionNo, canonical) == token {
		t.Fatalf("another tenant key never matches the token")
	}
}

func TestExternalContentDigestIsDeterministicAndOpaque(t *testing.T) {
	digest := ExternalContentDigest("Обещаем перезвонить")
	if len(digest) != 64 || !isHex64(digest) {
		t.Fatalf("the digest is the 64-hex SHA-256 print, got %q", digest)
	}
	if ExternalContentDigest("Обещаем перезвонить") != digest {
		t.Fatalf("the digest of the same content is deterministic")
	}
	if ExternalContentDigest("Обещаем перезвонить.") == digest {
		t.Fatalf("a changed content changes the digest")
	}
	if strings.Contains(digest, "Обещаем") {
		t.Fatalf("the digest never carries the content")
	}
}

// ---------------------------------------------------------------------------
// The readiness derivation (the ordered stale fences)
// ---------------------------------------------------------------------------

func extReadyInput() ExternalReadinessInput {
	version := extTestVersion()
	return ExternalReadinessInput{
		Now:               extTestNow(),
		MaxVersionNo:      version.VersionNo,
		OwnerEpochCurrent: version.OwnerEpoch,
		PinReadiness:      CommitmentReadinessConfirmable,
	}
}

func TestDeriveExternalReadinessReady(t *testing.T) {
	if got := DeriveExternalReadiness(extTestVersion(), extReadyInput()); got != ExternalReadinessReady {
		t.Fatalf("the live envelope with the confirmable pin is ready, got %q", got)
	}
}

func TestDeriveExternalReadinessSupersededWinsTheOrder(t *testing.T) {
	version := extTestVersion()
	input := extReadyInput()
	input.MaxVersionNo = 2                           // the newer version exists
	input.Now = version.ValidUntil.Add(time.Hour)    // the window is dead too
	input.OwnerEpochCurrent = version.OwnerEpoch + 1 // the owner moved too
	if got := DeriveExternalReadiness(version, input); got != ExternalReadinessSuperseded {
		t.Fatalf("the superseded fence wins the order, got %q", got)
	}
}

func TestDeriveExternalReadinessExpiredBeatsTheLaterFences(t *testing.T) {
	version := extTestVersion()
	input := extReadyInput()
	input.MaxVersionNo = version.VersionNo
	input.Now = version.ValidUntil // the boundary instant is not alive (fail-closed)
	input.OwnerEpochCurrent = version.OwnerEpoch + 1
	if got := DeriveExternalReadiness(version, input); got != ExternalReadinessExpired {
		t.Fatalf("the expired fence beats the owner fence, got %q", got)
	}
}

func TestDeriveExternalReadinessOwnerStaleBeatsThePinFence(t *testing.T) {
	version := extTestVersion()
	version.Purpose = ExternalPurposeOffer
	version.Pin = extTestPin()
	input := extReadyInput()
	input.OwnerEpochCurrent = version.OwnerEpoch + 5
	input.PinReadiness = CommitmentReadinessSuperseded
	if got := DeriveExternalReadiness(version, input); got != ExternalReadinessOwnerStale {
		t.Fatalf("the owner fence beats the pin fence, got %q", got)
	}
}

func TestDeriveExternalReadinessPinBlockedIsTheOfferFenceOnly(t *testing.T) {
	offer := extTestVersion()
	offer.Purpose = ExternalPurposeOffer
	offer.Pin = extTestPin()
	input := extReadyInput()
	input.PinReadiness = CommitmentReadinessSuperseded
	if got := DeriveExternalReadiness(offer, input); got != ExternalReadinessPinBlocked {
		t.Fatalf("the offer with the dead pin readiness is blocked, got %q", got)
	}
	input.PinReadiness = CommitmentReadinessDraft
	if got := DeriveExternalReadiness(offer, input); got != ExternalReadinessPinBlocked {
		t.Fatalf("the draft pin readiness blocks the offer too, got %q", got)
	}
	input.PinReadiness = CommitmentReadinessConfirmable
	if got := DeriveExternalReadiness(offer, input); got != ExternalReadinessReady {
		t.Fatalf("the offer with the confirmable pin is ready, got %q", got)
	}
	nonOffer := extTestVersion() // a promise: the pin readiness is never consulted
	input.PinReadiness = CommitmentReadinessSuperseded
	if got := DeriveExternalReadiness(nonOffer, input); got != ExternalReadinessReady {
		t.Fatalf("the non-offer purpose never consults the pin readiness, got %q", got)
	}
}

func TestGuardEnqueueOpensTheReadyPathOnly(t *testing.T) {
	if err := GuardEnqueueExternalCommitment(ExternalReadinessReady); err != nil {
		t.Fatalf("the ready word opens the enqueue, got %v", err)
	}
	for _, word := range []string{
		ExternalReadinessSuperseded, ExternalReadinessExpired,
		ExternalReadinessOwnerStale, ExternalReadinessPinBlocked,
	} {
		err := GuardEnqueueExternalCommitment(word)
		if err == nil {
			t.Fatalf("the refusal word %q never opens the enqueue", word)
		}
		if _, honest := ExternalReadinessRefusalTexts[word]; !honest {
			t.Fatalf("the refusal word %q carries its honest text", word)
		}
	}
	if err := GuardEnqueueExternalCommitment("no_such_readiness"); err == nil {
		t.Fatalf("the foreign readiness word fails closed, got nil")
	}
}

func TestExternalApprovalRequiredTextNamesTheExactVersion(t *testing.T) {
	text := ExternalApprovalRequiredText(7)
	if !strings.Contains(text, "7") || !strings.Contains(text, "владельцем") {
		t.Fatalf("the approval text names the role and the exact version, got %q", text)
	}
}

// ---------------------------------------------------------------------------
// The seven-state derivation (every transition by name)
// ---------------------------------------------------------------------------

func TestDeriveExternalDeliveryStatePersistedWithoutTheOutboxRecord(t *testing.T) {
	state := DeriveExternalDeliveryState(false, nil)
	if state.State != ExternalDeliveryPersisted {
		t.Fatalf("without the outbox record the truth is persisted, got %q", state.State)
	}
	// Even the stray events of an unqueued row never claim a send (the
	// defensive derivation of the 2.12 precedent).
	state = DeriveExternalDeliveryState(false, []ExternalDispatchEvent{extTestEvent(1, ExternalDispatchAttemptSubmitted)})
	if state.State != ExternalDeliveryPersisted {
		t.Fatalf("the events of an unqueued row never resurrect a send, got %q", state.State)
	}
}

func TestDeriveExternalDeliveryStateQueuedWithThePendingRecord(t *testing.T) {
	state := DeriveExternalDeliveryState(true, nil)
	if state.State != ExternalDeliveryQueued {
		t.Fatalf("the pending outbox record without events is queued, got %q", state.State)
	}
}

func TestDeriveExternalDeliveryStateSubmitted(t *testing.T) {
	event := extTestEvent(1, ExternalDispatchAttemptSubmitted)
	state := DeriveExternalDeliveryState(true, []ExternalDispatchEvent{event})
	if state.State != ExternalDeliverySubmitted {
		t.Fatalf("the submitted attempt is the submitted state, got %q", state.State)
	}
	if state.LastExternalCorrelationID != event.ExternalCorrelationID {
		t.Fatalf("the external correlation id of AC-8 stays in the state, got %q", state.LastExternalCorrelationID)
	}
}

func TestDeriveExternalDeliveryStateFailedAttempt(t *testing.T) {
	state := DeriveExternalDeliveryState(true, []ExternalDispatchEvent{extTestEvent(1, ExternalDispatchAttemptFailed)})
	if state.State != ExternalDeliveryFailed {
		t.Fatalf("the pre-send provider refusal is the failed state, got %q", state.State)
	}
}

func TestDeriveExternalDeliveryStateUnknownAttempt(t *testing.T) {
	event := extTestEvent(1, ExternalDispatchAttemptUnknown)
	state := DeriveExternalDeliveryState(true, []ExternalDispatchEvent{event})
	if state.State != ExternalDeliveryUnknown {
		t.Fatalf("the timeout after a possible send is the unknown state, got %q", state.State)
	}
	if state.LastExternalCorrelationID != event.ExternalCorrelationID {
		t.Fatalf("the unknown state keeps the correlation id of AC-8")
	}
}

func TestDeriveExternalDeliveryStateDeliveredReceipt(t *testing.T) {
	events := []ExternalDispatchEvent{
		extTestEvent(1, ExternalDispatchAttemptSubmitted),
		extTestEvent(2, ExternalDispatchReceiptDelivered),
	}
	state := DeriveExternalDeliveryState(true, events)
	if state.State != ExternalDeliveryDelivered {
		t.Fatalf("the delivered receipt is the delivered state, got %q", state.State)
	}
	if state.LastEventNo != 2 {
		t.Fatalf("the last event of the monotonic history decides, got %d", state.LastEventNo)
	}
}

func TestDeriveExternalDeliveryStateAcknowledgedReceipt(t *testing.T) {
	events := []ExternalDispatchEvent{
		extTestEvent(1, ExternalDispatchAttemptSubmitted),
		extTestEvent(2, ExternalDispatchReceiptDelivered),
		extTestEvent(3, ExternalDispatchReceiptAcknowledged),
	}
	state := DeriveExternalDeliveryState(true, events)
	if state.State != ExternalDeliveryAcknowledged {
		t.Fatalf("the acknowledged receipt is the acknowledged state, got %q", state.State)
	}
}

func TestDeriveExternalDeliveryStateFailedReceipt(t *testing.T) {
	events := []ExternalDispatchEvent{
		extTestEvent(1, ExternalDispatchAttemptSubmitted),
		extTestEvent(2, ExternalDispatchReceiptFailed),
	}
	state := DeriveExternalDeliveryState(true, events)
	if state.State != ExternalDeliveryFailed {
		t.Fatalf("the operator-relayed refusal is the failed state, got %q", state.State)
	}
}

func TestDeriveExternalDeliveryStateCancelledStaleReturnsToPersisted(t *testing.T) {
	event := extTestEvent(1, ExternalDispatchCancelledStale)
	event.Reason = ExternalStaleNewerVersion
	state := DeriveExternalDeliveryState(true, []ExternalDispatchEvent{event})
	if state.State != ExternalDeliveryPersisted {
		t.Fatalf("the stale fence killed the queue BEFORE the words left — the honest state is persisted, got %q", state.State)
	}
	if state.LastKind != ExternalDispatchCancelledStale || state.LastReason != ExternalStaleNewerVersion {
		t.Fatalf("the cancellation material stays in the state for the page, got kind %q reason %q", state.LastKind, state.LastReason)
	}
}

func TestDeriveExternalDeliveryStateLastEventDecidesRegardlessOfSliceOrder(t *testing.T) {
	events := []ExternalDispatchEvent{
		extTestEvent(3, ExternalDispatchReceiptDelivered),
		extTestEvent(1, ExternalDispatchAttemptSubmitted),
		extTestEvent(2, ExternalDispatchAttemptUnknown),
	}
	state := DeriveExternalDeliveryState(true, events)
	if state.State != ExternalDeliveryDelivered || state.LastEventNo != 3 {
		t.Fatalf("the maximal event number decides, not the slice order, got %q no %d", state.State, state.LastEventNo)
	}
}

func TestDeriveExternalDeliveryStateForeignKindFailsToUnknown(t *testing.T) {
	state := DeriveExternalDeliveryState(true, []ExternalDispatchEvent{extTestEvent(1, "happily_delivered")})
	if state.State != ExternalDeliveryUnknown {
		t.Fatalf("the foreign kind (impossible through the guard and the CHECK) fails safely to unknown, got %q", state.State)
	}
}

func TestExternalDeliveryStateTextsAreDistinctAndHonest(t *testing.T) {
	seen := make(map[string]string)
	for _, word := range ExternalDeliveryStateWords {
		text := ExternalDeliveryStateText(word)
		if strings.TrimSpace(text) == "" {
			t.Fatalf("the state %q carries its render text", word)
		}
		for other, otherText := range seen {
			if otherText == text {
				t.Fatalf("the states %q and %q never share a text", word, other)
			}
		}
		seen[word] = text
	}
	submitted := ExternalDeliveryStateText(ExternalDeliverySubmitted)
	if !strings.Contains(submitted, "НЕТ") {
		t.Fatalf("the submitted text says there is NO delivery nor acceptance, got %q", submitted)
	}
	if strings.Contains(submitted, ExternalDeliveryStateText(ExternalDeliveryAcknowledged)) {
		t.Fatalf("the submitted text never reads as the acceptance")
	}
	unknown := ExternalDeliveryStateText(ExternalDeliveryUnknown)
	if !strings.Contains(unknown, "reconciliation") {
		t.Fatalf("the unknown text names the reconciliation path (no blind retry), got %q", unknown)
	}
}

// ---------------------------------------------------------------------------
// The relay, outcome and stale-reason guards
// ---------------------------------------------------------------------------

func TestGuardRelayObservationRefusesTheInventedOutcome(t *testing.T) {
	if err := GuardRelayObservation(RelayObservationDelivered); err != nil {
		t.Fatalf("the honest delivered observation passes, got %v", err)
	}
	if err := GuardRelayObservation("probably_delivered"); err == nil {
		t.Fatalf("the invented outcome never passes, got nil")
	}
}

func TestRelayObservationDispatchKindMapping(t *testing.T) {
	cases := map[string]string{
		RelayObservationDelivered:    ExternalDispatchReceiptDelivered,
		RelayObservationAcknowledged: ExternalDispatchReceiptAcknowledged,
		RelayObservationFailed:       ExternalDispatchReceiptFailed,
	}
	for word, kind := range cases {
		if RelayObservationDispatchKind(word) != kind {
			t.Fatalf("the observation %q maps to the kind %q", word, kind)
		}
	}
	if RelayObservationDispatchKind("invented") != "" {
		t.Fatalf("the foreign observation maps to nothing (the guard refuses it first)")
	}
}

func TestGuardDispatchOutcomeRefusesTheForeignWord(t *testing.T) {
	if err := GuardDispatchOutcome(DispatchOutcomeTimeout); err != nil {
		t.Fatalf("the timeout outcome belongs to the adapter contract, got %v", err)
	}
	if err := GuardDispatchOutcome("vanished"); err == nil {
		t.Fatalf("the foreign outcome is impossible, got nil")
	}
}

func TestGuardExternalStaleReasonRefusesTheForeignWord(t *testing.T) {
	if err := GuardExternalStaleReason(ExternalStaleControlMode); err != nil {
		t.Fatalf("the control mode reason is the live 3.2 enforcement word, got %v", err)
	}
	if err := GuardExternalStaleReason("felt_stale"); err == nil {
		t.Fatalf("the foreign reason is impossible, got nil")
	}
}

// ---------------------------------------------------------------------------
// The outbox perimeter (the material never leaves the row, AC-10)
// ---------------------------------------------------------------------------

func TestQueuedOutboxPayloadNeverCarriesTheContent(t *testing.T) {
	version := extTestVersion()
	payload := ExternalCommitmentQueuedPayload{
		TenantID:        version.TenantID,
		CaseID:          version.CaseID,
		ExtCommitmentID: version.ExtCommitmentID,
		VersionNo:       version.VersionNo,
		Purpose:         version.Purpose,
		RecipientKind:   version.RecipientKind,
		Channel:         version.Channel,
		IdempotencyKey:  ExternalCommitmentIdempotencyKey(version.TenantID, version.ExtCommitmentID, version.VersionNo),
		ContentDigest:   ExternalContentDigest(version.ContentText),
	}
	rendered, err := ExternalCommitmentQueuedPayloadJSON(payload)
	if err != nil {
		t.Fatalf("the safe payload renders, got %v", err)
	}
	if bytes.Contains(rendered, []byte(version.ContentText)) {
		t.Fatalf("the content never enters the outbox envelope, got %s", rendered)
	}
	if !bytes.Contains(rendered, []byte(version.Channel)) || !bytes.Contains(rendered, []byte(ExternalContentDigest(version.ContentText))) {
		t.Fatalf("the safe envelope carries the ids, the words and the digest, got %s", rendered)
	}
}

func TestDedupKeyExternalCommitmentQueuedIsTheVersionPin(t *testing.T) {
	first := DedupKeyExternalCommitmentQueued(extTestID(), 1)
	if DedupKeyExternalCommitmentQueued(extTestID(), 1) != first {
		t.Fatalf("the dedup key of the same version is deterministic")
	}
	if DedupKeyExternalCommitmentQueued(extTestID(), 2) == first {
		t.Fatalf("another version never shares the dedup key (no second intent)")
	}
	other := id.MustParse("019a2f4c-2900-7000-8000-0000000000ef")
	if DedupKeyExternalCommitmentQueued(other, 1) == first {
		t.Fatalf("another envelope never shares the dedup key")
	}
}

// ---------------------------------------------------------------------------
// The idempotency key (the HMAC derivative of the version pin)
// ---------------------------------------------------------------------------

func TestExternalCommitmentIdempotencyKeyIsTheVersionPinDerivative(t *testing.T) {
	tenant, ext := extTestTenant(), extTestID()
	key := ExternalCommitmentIdempotencyKey(tenant, ext, 1)
	if len(key) != 64 || !isHex64(key) {
		t.Fatalf("the idempotency key is the 64-hex HMAC print, got %q", key)
	}
	if ExternalCommitmentIdempotencyKey(tenant, ext, 1) != key {
		t.Fatalf("the repeated delivery of the SAME version carries the SAME key (the provider dedup)")
	}
	if ExternalCommitmentIdempotencyKey(tenant, ext, 2) == key {
		t.Fatalf("a changed version NEVER reuses the old key")
	}
	otherTenant := id.MustParse("019a2f4c-2900-7000-8000-0000000000ab")
	if ExternalCommitmentIdempotencyKey(otherTenant, ext, 1) == key {
		t.Fatalf("another tenant never shares the key")
	}
	otherExt := id.MustParse("019a2f4c-2900-7000-8000-0000000000ef")
	if ExternalCommitmentIdempotencyKey(tenant, otherExt, 1) == key {
		t.Fatalf("another envelope never shares the key")
	}
	if strings.Contains(key, tenant.String()) || strings.Contains(key, ext.String()) {
		t.Fatalf("the key is opaque — the identifiers never leak to the provider, got %q", key)
	}
	if key == ExternalCommitmentToken([]byte(tenant.String()), ext, 1, []byte("x")) {
		t.Fatalf("the idempotency space is separated from the token space")
	}
}

// ---------------------------------------------------------------------------
// The outbound payload (the determinism of the immutable row)
// ---------------------------------------------------------------------------

func TestBuildOutboundPayloadIsDeterministicByteToByte(t *testing.T) {
	version := extTestVersion()
	key := ExternalCommitmentIdempotencyKey(version.TenantID, version.ExtCommitmentID, version.VersionNo)
	first, err := json.Marshal(BuildOutboundExternalPayload(version, key))
	if err != nil {
		t.Fatalf("the outbound payload renders, got %v", err)
	}
	second, err := json.Marshal(BuildOutboundExternalPayload(extTestVersion(), ExternalCommitmentIdempotencyKey(version.TenantID, version.ExtCommitmentID, version.VersionNo)))
	if err != nil {
		t.Fatalf("the second build renders, got %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("the same immutable row always yields the same payload byte-to-byte")
	}
}

func TestBuildOutboundPayloadSurvivesTheLaterDraftChange(t *testing.T) {
	// A later draft change is a NEW row — the queued payload of the pinned
	// version never drifts (AC-4: the determinism comes from the
	// immutability, never from a snapshot).
	queued := extTestVersion()
	key := ExternalCommitmentIdempotencyKey(queued.TenantID, queued.ExtCommitmentID, queued.VersionNo)
	before, err := json.Marshal(BuildOutboundExternalPayload(queued, key))
	if err != nil {
		t.Fatalf("the queued payload renders, got %v", err)
	}
	newDraft := extTestVersion()
	newDraft.VersionNo = 2
	newDraft.ContentText = "Совершенно изменённый текст второй версии"
	after, err := json.Marshal(BuildOutboundExternalPayload(queued, key))
	if err != nil {
		t.Fatalf("the re-build of the SAME row renders, got %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("the new version row never alters the queued payload of the old version")
	}
	other, err := json.Marshal(BuildOutboundExternalPayload(newDraft, ExternalCommitmentIdempotencyKey(newDraft.TenantID, newDraft.ExtCommitmentID, newDraft.VersionNo)))
	if err != nil {
		t.Fatalf("the new row builds its own payload, got %v", err)
	}
	if bytes.Equal(before, other) {
		t.Fatalf("the new version carries its own material")
	}
}

func TestBuildOutboundPayloadCarriesTheDeliveryMaterial(t *testing.T) {
	version := extTestVersion()
	key := ExternalCommitmentIdempotencyKey(version.TenantID, version.ExtCommitmentID, version.VersionNo)
	payload := BuildOutboundExternalPayload(version, key)
	if payload.ContentText != version.ContentText {
		t.Fatalf("the outbound payload carries the words being delivered")
	}
	if payload.ExtCommitmentID != version.ExtCommitmentID || payload.VersionNo != version.VersionNo {
		t.Fatalf("the payload pins the exact version")
	}
	if payload.Channel != version.Channel || payload.RecipientKind != version.RecipientKind {
		t.Fatalf("the payload carries the route material")
	}
	if payload.AckRequirement != version.AckRequirement || payload.ExpectedNextStep != version.ExpectedNextStep {
		t.Fatalf("the payload carries the ack requirement and the next step")
	}
	if payload.IdempotencyKey != key {
		t.Fatalf("the payload carries the idempotency key of the version pin")
	}
	if payload.ValidUntil != version.ValidUntil.UTC() {
		t.Fatalf("the payload carries the UTC validity")
	}
}

// ---------------------------------------------------------------------------
// The current version resolution
// ---------------------------------------------------------------------------

func TestCurrentExternalCommitmentVersionResolvesTheMaximum(t *testing.T) {
	if CurrentExternalCommitmentVersion(nil) != nil {
		t.Fatalf("the empty history has no current version")
	}
	first := extTestVersion()
	second := extTestVersion()
	second.VersionNo = 2
	third := extTestVersion()
	third.VersionNo = 3
	// The slice order never matters — the maximum wins.
	current := CurrentExternalCommitmentVersion([]ExternalCommitment{second, first, third})
	if current == nil || current.VersionNo != 3 {
		t.Fatalf("the maximal version number is the current version, got %+v", current)
	}
}
