package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/platform/id"
)

// modeBase builds the deterministic material of the mode tests: a
// conversation with the ingest initial term (version 1, unclassified)
// over the fixed clock word.
func modeBase() ([]ModeTerm, time.Time) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	terms := []ModeTerm{{
		TenantID:       id.MustParse("00000000-0000-7000-8000-0000000000a1"),
		CaseID:         id.MustParse("00000000-0000-7000-8000-0000000000b1"),
		ConversationID: id.MustParse("00000000-0000-7000-8000-000000000011"),
		TermID:         id.MustParse("00000000-0000-7000-8000-0000000000c1"),
		ModeVersion:    1, Mode: ModeUnclassified, Scope: ModeScopeConversation,
		Origin: ModeOriginIngestInitial, Reason: "Запрос ещё не классифицирован.",
		SignalSource: ModeSignalOperator, ActorID: id.MustParse("00000000-0000-7000-8000-0000000000d1"),
		RuleVersion: ModePolicyVersion, CreatedAt: now.Add(-time.Hour),
	}}
	return terms, now
}

func TestModeLadderDictionaryIsFixed(t *testing.T) {
	if len(ModeOrder) != 7 {
		t.Fatalf("the ladder carries exactly seven modes, got %d", len(ModeOrder))
	}
	for rank, mode := range ModeOrder {
		if ModeRank(mode) != rank {
			t.Fatalf("the rank of %s must be %d", mode, rank)
		}
	}
	if ModeRank("unknown") != -1 {
		t.Fatalf("the unknown mode has no rank, got %d", ModeRank("unknown"))
	}
	for _, mode := range ModeOrder {
		if ModeIsNonSales(mode) != (mode != ModeSales) {
			t.Fatalf("the non-sales word of %s is wrong", mode)
		}
	}
}

func TestActiveConversationModeIsMaxVersion(t *testing.T) {
	terms, _ := modeBase()
	active := ActiveConversationMode(terms)
	if active == nil || active.ModeVersion != 1 || active.Mode != ModeUnclassified {
		t.Fatalf("expected the version 1 unclassified term, got %+v", active)
	}
	// The Work Item scope never feeds the conversation derivation.
	terms = append(terms, ModeTerm{
		ConversationID: terms[0].ConversationID, TermID: id.MustParse("00000000-0000-7000-8000-0000000000c2"),
		ModeVersion: 2, Mode: ModeCancellation, Scope: ModeScopeWorkItem,
		WorkItemID: id.MustParse("00000000-0000-7000-8000-0000000000e9"),
		Origin:     ModeOriginProposal, RuleVersion: ModePolicyVersion,
	})
	if active = ActiveConversationMode(terms); active.ModeVersion != 1 {
		t.Fatalf("the work item term never raises the conversation mode, got %+v", active)
	}
	terms = append(terms, ModeTerm{
		ConversationID: terms[0].ConversationID, TermID: id.MustParse("00000000-0000-7000-8000-0000000000c3"),
		ModeVersion: 3, Mode: ModeSales, Scope: ModeScopeConversation,
		Origin: ModeOriginProposal, RuleVersion: ModePolicyVersion, PriorMode: ModeUnclassified,
	})
	if active = ActiveConversationMode(terms); active.ModeVersion != 3 || active.Mode != ModeSales {
		t.Fatalf("expected the version 3 sales term, got %+v", active)
	}
	if ActiveConversationMode(nil) != nil {
		t.Fatal("the empty ladder has no active term")
	}
}

func TestWorkItemContextAndEffectiveMode(t *testing.T) {
	terms, _ := modeBase()
	wi := id.MustParse("00000000-0000-7000-8000-0000000000e9")
	if context := WorkItemModeContext(terms, wi); context != nil {
		t.Fatalf("the item without terms has no context, got %+v", context)
	}
	terms = append(terms, ModeTerm{
		ConversationID: terms[0].ConversationID, TermID: id.MustParse("00000000-0000-7000-8000-0000000000c2"),
		ModeVersion: 2, Mode: ModeCancellation, Scope: ModeScopeWorkItem, WorkItemID: wi,
		Origin: ModeOriginProposal, RuleVersion: ModePolicyVersion,
	})
	context := WorkItemModeContext(terms, wi)
	if context == nil || context.Mode != ModeCancellation {
		t.Fatalf("expected the cancellation context, got %+v", context)
	}
	// The effective mode of the item: the stricter of the conversation word
	// (unclassified) and the item word (cancellation) — cancellation wins.
	active := ActiveConversationMode(terms)
	if effective := EffectiveWorkMode(active, context); effective.Mode != ModeCancellation {
		t.Fatalf("expected the stricter cancellation word, got %+v", effective)
	}
	// The item without a context: the conversation word applies.
	if effective := EffectiveWorkMode(active, nil); effective.Mode != ModeUnclassified {
		t.Fatalf("expected the conversation word, got %+v", effective)
	}
	// The pre-migration absence degrades to the fail-closed unclassified word.
	if EffectiveWorkModeWord(nil) != ModeUnclassified {
		t.Fatal("the absent material is the unclassified fail-closed word")
	}
	// The safety conversation beats a sales item context (AC-8: the
	// conversation applies the strictest mode to the incompatible items).
	safety := &ModeTerm{Mode: ModeSafety}
	salesContext := &ModeTerm{Mode: ModeSales}
	if effective := EffectiveWorkMode(safety, salesContext); effective.Mode != ModeSafety {
		t.Fatalf("the safety conversation wins, got %+v", effective)
	}
}

func TestModeTermMaxVersionCountsEveryScope(t *testing.T) {
	terms, _ := modeBase()
	if max := ModeTermMaxVersion(terms); max != 1 {
		t.Fatalf("expected the max version 1, got %d", max)
	}
	terms = append(terms, ModeTerm{
		ConversationID: terms[0].ConversationID, ModeVersion: 2, Scope: ModeScopeWorkItem,
		WorkItemID: id.MustParse("00000000-0000-7000-8000-0000000000e9"), Mode: ModeClaim,
	})
	if max := ModeTermMaxVersion(terms); max != 2 {
		t.Fatalf("every scope consumes a version: expected 2, got %d", max)
	}
}

func TestGuardProposeTransitionLadder(t *testing.T) {
	terms, _ := modeBase()
	active := ActiveConversationMode(terms)
	// The raise: unclassified -> sales passes.
	if err := GuardProposeTransition(ModeScopeConversation, ModeSales, active, nil); err != nil {
		t.Fatalf("the raise must pass, got %v", err)
	}
	// The equal repeat: the honest mode_active refusal.
	err := GuardProposeTransition(ModeScopeConversation, ModeUnclassified, active, nil)
	if err == nil || !strings.Contains(err.Error(), "уже активен") {
		t.Fatalf("expected the mode_active refusal, got %v", err)
	}
	// The downgrade: no surface can ever lower by proposing.
	err = GuardProposeTransition(ModeScopeConversation, ModeSales, &ModeTerm{Mode: ModeSafety}, nil)
	if err == nil || !strings.Contains(err.Error(), "понизить") {
		t.Fatalf("expected the mode_not_downgradable refusal, got %v", err)
	}
	// The Work Item scope decides against the item context: a first context
	// is free (it lowers nothing).
	if err := GuardProposeTransition(ModeScopeWorkItem, ModeCancellation, nil, nil); err != nil {
		t.Fatalf("the first context is free, got %v", err)
	}
	// The item context repeat refuses; the lower word refuses.
	context := &ModeTerm{Mode: ModeCancellation}
	if err := GuardProposeTransition(ModeScopeWorkItem, ModeCancellation, nil, context); err == nil {
		t.Fatal("the item context repeat refuses")
	}
	if err := GuardProposeTransition(ModeScopeWorkItem, ModeSales, nil, context); err == nil {
		t.Fatal("the item context downgrade refuses")
	}
	if err := GuardProposeTransition(ModeScopeWorkItem, ModeSafety, nil, context); err != nil {
		t.Fatalf("the item context raise passes, got %v", err)
	}
}

func TestGuardExitTransitionLadder(t *testing.T) {
	// The exit lowers: safety -> sales passes.
	if err := GuardExitTransition(ModeSales, &ModeTerm{Mode: ModeSafety}); err != nil {
		t.Fatalf("the lowering exit must pass, got %v", err)
	}
	// The equal word refuses.
	if err := GuardExitTransition(ModeSafety, &ModeTerm{Mode: ModeSafety}); err == nil {
		t.Fatal("the equal exit refuses")
	}
	// The raising exit refuses with the honest propose path.
	err := GuardExitTransition(ModeSafety, &ModeTerm{Mode: ModeSales})
	if err == nil || !strings.Contains(err.Error(), "propose_mode") {
		t.Fatalf("expected the propose path refusal, got %v", err)
	}
	// The exit to unclassified is a valid lowering (the honest
	// un-classification word of the admission floor).
	if err := GuardExitTransition(ModeUnclassified, &ModeTerm{Mode: ModeSales}); err != nil {
		t.Fatalf("the un-classification exit passes, got %v", err)
	}
	// The empty ladder refuses honestly.
	err = GuardExitTransition(ModeSales, nil)
	if err == nil || !strings.Contains(err.Error(), "до-миграционные") {
		t.Fatalf("expected the empty ladder refusal, got %v", err)
	}
}

func TestExitAuthorityLadder(t *testing.T) {
	// The safety exit is the supervision word only (AC-9).
	if ExitAuthorityAllowed(ModeSafety, true, false) {
		t.Fatal("the owner never exits the safety mode")
	}
	if !ExitAuthorityAllowed(ModeSafety, false, true) {
		t.Fatal("the supervision exits the safety mode")
	}
	// The other modes: the current owner or the supervision.
	if !ExitAuthorityAllowed(ModeClaim, true, false) {
		t.Fatal("the owner exits the claim mode")
	}
	if !ExitAuthorityAllowed(ModeClaim, false, true) {
		t.Fatal("the supervision exits the claim mode")
	}
	if ExitAuthorityAllowed(ModeClaim, false, false) {
		t.Fatal("a bystander never exits any mode")
	}
}

func TestModeCommandGuards(t *testing.T) {
	if err := GuardMode("no-such-mode"); err == nil || !strings.Contains(err.Error(), "словарь") {
		t.Fatalf("expected the dictionary refusal, got %v", err)
	}
	if err := GuardMode(ModeSales); err != nil {
		t.Fatalf("the dictionary mode passes, got %v", err)
	}
	if err := GuardModeScope("case"); err == nil {
		t.Fatal("the unknown scope refuses")
	}
	if err := GuardModeReason(strings.Repeat("а", MaxModeReasonRunes+1)); err == nil {
		t.Fatal("the oversized reason refuses")
	}
	// The signal discipline: the turn source needs the turn reference; the
	// other words carry none.
	turn := id.MustParse("00000000-0000-7000-8000-0000000000e1")
	if err := GuardSignalSource(ModeSignalTurnEvent, id.UUID("")); err == nil {
		t.Fatal("the turn source without the reference refuses")
	}
	if err := GuardSignalSource(ModeSignalTurnEvent, turn); err != nil {
		t.Fatalf("the turn source with the reference passes, got %v", err)
	}
	if err := GuardSignalSource(ModeSignalOperator, turn); err == nil {
		t.Fatal("the operator word never carries the turn reference")
	}
	if err := GuardSignalSource("customer", id.UUID("")); err == nil {
		t.Fatal("the unknown source refuses")
	}
	if err := GuardExpectedModeVersion(0); err == nil {
		t.Fatal("the zero expectation refuses")
	}
	// The scope material discipline.
	if err := GuardProposeModeMaterial(ModeScopeWorkItem, ModeSales, "r", id.UUID("")); err == nil {
		t.Fatal("the work item scope without the item refuses")
	}
	if err := GuardProposeModeMaterial(ModeScopeConversation, ModeSales, "r", id.MustParse("00000000-0000-7000-8000-0000000000e9")); err == nil {
		t.Fatal("the conversation scope never carries the item")
	}
	if err := GuardProposeModeMaterial(ModeScopeConversation, ModeSales, "r", id.UUID("")); err != nil {
		t.Fatalf("the honest conversation proposal passes, got %v", err)
	}
	// The exit material: the mandatory reason and the authority dictionary.
	if err := GuardExitModeMaterial(ModeSales, "", ModeAuthoritySupervisor); err == nil {
		t.Fatal("the empty exit reason refuses")
	}
	if err := GuardExitModeMaterial(ModeSales, "r", "root"); err == nil {
		t.Fatal("the unknown authority refuses")
	}
	if err := GuardExitModeMaterial(ModeSales, "r", ModeAuthorityOperatorOwner); err != nil {
		t.Fatalf("the authority word passes, got %v", err)
	}
	// The reply class: the dictionary without a silent default.
	for _, class := range ReplyClasses {
		if err := GuardReplyClass(class); err != nil {
			t.Fatalf("the class %s passes, got %v", class, err)
		}
	}
	if err := GuardReplyClass(""); err == nil {
		t.Fatal("the empty class refuses honestly")
	}
	if err := GuardReplyClass("marketing"); err == nil {
		t.Fatal("the unknown class refuses")
	}
	// The disclosure material.
	if err := GuardDisclosureText(""); err == nil {
		t.Fatal("the empty disclosure text refuses")
	}
	if err := GuardDisclosureText(strings.Repeat("а", MaxDisclosureTextRunes+1)); err == nil {
		t.Fatal("the oversized disclosure text refuses")
	}
	if err := GuardDisclosureChannel("no-such-channel"); err == nil {
		t.Fatal("the unknown channel refuses")
	}
	if err := GuardDisclosureChannel("phone"); err != nil {
		t.Fatalf("the dictionary channel passes, got %v", err)
	}
	if err := GuardDeliveryKind("lost"); err == nil {
		t.Fatal("the unknown delivery kind refuses")
	}
	if err := GuardDeliveryKind(DeliveryInterrupted); err != nil {
		t.Fatalf("the dictionary kind passes, got %v", err)
	}
}

func TestGuardReplyClassAgainstModeGate(t *testing.T) {
	// The sales class needs the effective sales mode.
	if err := GuardReplyClassAgainstMode(ReplyClassSales, &ModeTerm{Mode: ModeSales}); err != nil {
		t.Fatalf("the sales reply under the sales mode passes, got %v", err)
	}
	err := GuardReplyClassAgainstMode(ReplyClassSales, &ModeTerm{Mode: ModeSafety})
	var gate ReplyModeError
	if !errors.As(err, &gate) || gate.EffectiveMode != ModeSafety {
		t.Fatalf("expected the machine-readable mode refusal, got %v", err)
	}
	// The pre-migration absence is the fail-closed non-sales word.
	err = GuardReplyClassAgainstMode(ReplyClassSales, nil)
	if !errors.As(err, &gate) || gate.EffectiveMode != ModeUnclassified {
		t.Fatalf("expected the fail-closed unclassified refusal, got %v", err)
	}
	// The neutral and the protected classes never touch the mode gate.
	if err := GuardReplyClassAgainstMode(ReplyClassNeutral, &ModeTerm{Mode: ModeSafety}); err != nil {
		t.Fatalf("the neutral reply passes under any mode, got %v", err)
	}
	if err := GuardReplyClassAgainstMode(ReplyClassProtectedRequest, &ModeTerm{Mode: ModeSafety}); err != nil {
		t.Fatalf("the protected class is a disclosure matter, got %v", err)
	}
}

// errorAs is the local errors.As shorthand.
func errorAs(err error, target any) bool { return errors.As(err, target) }

func TestGuardOutboundBasisFailClosed(t *testing.T) {
	// The full synthetic word passes (the positive branch — the producers
	// are AG-4; the unit proof over the synthetic words).
	if !GuardOutboundBasis(OutboundBasis{PartnerSource: "заявка на сайте", Purpose: "ответ на обращение", Consent: "согласие дано"}) {
		t.Fatal("the full basis word passes")
	}
	// Every missing word blocks the contact (fail-closed, AC-3).
	if GuardOutboundBasis(OutboundBasis{}) {
		t.Fatal("the empty basis blocks")
	}
	if GuardOutboundBasis(OutboundBasis{PartnerSource: "заявка", Purpose: "ответ"}) {
		t.Fatal("the missing consent blocks")
	}
	if GuardOutboundBasis(OutboundBasis{PartnerSource: " ", Purpose: "ответ", Consent: "да"}) {
		t.Fatal("the blank source word blocks")
	}
}

func TestDisclosureStatusAndEffectiveDerivation(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	tenant := id.MustParse("00000000-0000-7000-8000-0000000000a1")
	conversation := id.MustParse("00000000-0000-7000-8000-000000000011")
	disclosure := id.MustParse("00000000-0000-7000-8000-0000000000f1")
	records := []DisclosureRecord{{
		TenantID: tenant, ConversationID: conversation, DisclosureID: disclosure, DisclosureNo: 1,
		Channel: "phone", Text: "Я цифровой сотрудник компании.", CreatedAt: now,
	}}
	events := []DisclosureDeliveryEvent{{
		TenantID: tenant, EventID: id.MustParse("00000000-0000-7000-8000-0000000000f2"),
		DisclosureID: disclosure, EventNo: 1, Kind: DeliveryGenerated, CreatedAt: now,
	}}
	// The fresh record does not act yet (generated).
	if DisclosureStatus(events, disclosure) != DeliveryGenerated {
		t.Fatal("the born record is generated")
	}
	if effective := EffectiveDisclosure(records, events, conversation, "phone"); effective != nil {
		t.Fatalf("the generated record does not act, got %+v", effective)
	}
	// The sent word acts.
	events = append(events, DisclosureDeliveryEvent{
		TenantID: tenant, EventID: id.MustParse("00000000-0000-7000-8000-0000000000f3"),
		DisclosureID: disclosure, EventNo: 2, Kind: DeliverySent, CreatedAt: now.Add(time.Minute),
	})
	if effective := EffectiveDisclosure(records, events, conversation, "phone"); effective == nil {
		t.Fatal("the sent record acts for the channel")
	}
	// The playback words act too (never a proof of hearing — AC-2).
	events = append(events, DisclosureDeliveryEvent{
		TenantID: tenant, EventID: id.MustParse("00000000-0000-7000-8000-0000000000f4"),
		DisclosureID: disclosure, EventNo: 3, Kind: DeliveryPlaybackEstimated, CreatedAt: now.Add(2 * time.Minute),
	})
	if effective := EffectiveDisclosure(records, events, conversation, "phone"); effective == nil {
		t.Fatal("the playback word acts")
	}
	// The interruption voids the record: the last event wins.
	events = append(events, DisclosureDeliveryEvent{
		TenantID: tenant, EventID: id.MustParse("00000000-0000-7000-8000-0000000000f5"),
		DisclosureID: disclosure, EventNo: 4, Kind: DeliveryInterrupted, CreatedAt: now.Add(3 * time.Minute),
	})
	if DisclosureStatus(events, disclosure) != DeliveryInterrupted {
		t.Fatal("the last event is the record status")
	}
	if effective := EffectiveDisclosure(records, events, conversation, "phone"); effective != nil {
		t.Fatalf("the interrupted record is void, got %+v", effective)
	}
	// The re-disclosure with the parent reference acts again after sent.
	reDisclosure := id.MustParse("00000000-0000-7000-8000-0000000000f6")
	records = append(records, DisclosureRecord{
		TenantID: tenant, ConversationID: conversation, DisclosureID: reDisclosure, DisclosureNo: 2,
		Channel: "phone", Text: "Я цифровой сотрудник компании (повторно).", ParentRef: disclosure,
		CreatedAt: now.Add(4 * time.Minute),
	})
	events = append(events, DisclosureDeliveryEvent{
		TenantID: tenant, EventID: id.MustParse("00000000-0000-7000-8000-0000000000f7"),
		DisclosureID: reDisclosure, EventNo: 1, Kind: DeliverySent, CreatedAt: now.Add(5 * time.Minute),
	})
	if effective := EffectiveDisclosure(records, events, conversation, "phone"); effective == nil || effective.DisclosureID != reDisclosure {
		t.Fatalf("the re-disclosure acts, got %+v", effective)
	}
	// The other channel never sees this disclosure.
	if effective := EffectiveDisclosure(records, events, conversation, "web_chat"); effective != nil {
		t.Fatalf("the channel boundary holds, got %+v", effective)
	}
	// The other conversation never sees it either.
	if effective := EffectiveDisclosure(records, events, id.MustParse("00000000-0000-7000-8000-000000000012"), "phone"); effective != nil {
		t.Fatalf("the conversation boundary holds, got %+v", effective)
	}
	// The record without events carries the honest empty status.
	if DisclosureStatus(nil, disclosure) != "" {
		t.Fatal("the pre-migration record carries the empty status")
	}
}

func TestInitialModeTermShape(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	term := InitialModeTerm(
		id.MustParse("00000000-0000-7000-8000-0000000000a1"),
		id.MustParse("00000000-0000-7000-8000-0000000000b1"),
		id.MustParse("00000000-0000-7000-8000-000000000011"),
		id.MustParse("00000000-0000-7000-8000-0000000000c1"),
		id.MustParse("00000000-0000-7000-8000-0000000000d1"),
		now,
	)
	if term.ModeVersion != 1 || term.Mode != ModeUnclassified || term.Scope != ModeScopeConversation {
		t.Fatalf("the initial term is the version 1 unclassified conversation word, got %+v", term)
	}
	if term.Origin != ModeOriginIngestInitial || term.RuleVersion != ModePolicyVersion {
		t.Fatalf("the initial term carries the ingest origin and the rule version, got %+v", term)
	}
	if term.PriorMode != "" || term.ExitAuthority != "" {
		t.Fatalf("the initial term carries no prior mode and no authority, got %+v", term)
	}
	// The active derivation reads it immediately.
	if active := ActiveConversationMode([]ModeTerm{term}); active == nil || active.Mode != ModeUnclassified {
		t.Fatalf("the initial term is the active mode, got %+v", active)
	}
}

func TestModeOutboxPayloadsCarryNoDisclosureText(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	tenant := id.MustParse("00000000-0000-7000-8000-0000000000a1")
	caseID := id.MustParse("00000000-0000-7000-8000-0000000000b1")
	conversation := id.MustParse("00000000-0000-7000-8000-000000000011")
	secret := "СЕКРЕТНЫЙ-ТЕКСТ-РАСКРЫТИЯ"

	changed, err := ModeChangedPayloadJSON(ModeChangedPayload{
		TenantID: tenant, CaseID: caseID, ConversationID: conversation,
		TermID:      id.MustParse("00000000-0000-7000-8000-0000000000c2"),
		ModeVersion: 2, Mode: ModeSafety, Scope: ModeScopeConversation,
		Reason: "сигнал безопасности", SignalSource: ModeSignalTurnEvent,
		ActorID: id.MustParse("00000000-0000-7000-8000-0000000000d1"), OccurredAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(changed), secret) {
		t.Fatalf("the mode event never carries the disclosure text, got %s", changed)
	}
	var decoded map[string]any
	if err := json.Unmarshal(changed, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tenant_id", "case_id", "conversation_id", "term_id", "mode_version", "mode", "scope", "reason", "signal_source", "actor_id"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("the mode event must carry %q", field)
		}
	}

	recorded, err := DisclosureRecordedPayloadJSON(DisclosureRecordedPayload{
		TenantID: tenant, CaseID: caseID, ConversationID: conversation,
		DisclosureID: id.MustParse("00000000-0000-7000-8000-0000000000f1"),
		DisclosureNo: 1, Channel: "phone",
		ActorID: id.MustParse("00000000-0000-7000-8000-0000000000d1"), OccurredAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(recorded), secret) || strings.Contains(string(recorded), "text") {
		t.Fatalf("the disclosure event never carries the text, got %s", recorded)
	}
	var decodedRecord map[string]any
	if err := json.Unmarshal(recorded, &decodedRecord); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tenant_id", "case_id", "conversation_id", "disclosure_id", "disclosure_no", "channel"} {
		if _, ok := decodedRecord[field]; !ok {
			t.Fatalf("the disclosure event must carry %q", field)
		}
	}
}

func TestModeConflictErrorShape(t *testing.T) {
	err := ModeConflictError{ActiveMode: ModeSafety, Reason: "r", SignalSource: ModeSignalOperator, RuleVersion: ModePolicyVersion, ModeVersion: 3}
	if err.Error() != ModeConflictText {
		t.Fatalf("the conflict carries the honest text, got %s", err.Error())
	}
	var gate ReplyModeError
	if !errorAs(GuardReplyClassAgainstMode(ReplyClassSales, &ModeTerm{Mode: ModePrivacyNoContact}), &gate) {
		t.Fatal("the mode gate refusal is machine-readable")
	}
}
