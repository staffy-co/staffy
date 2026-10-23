package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// The Story 2.13 unit matrix (the domain half): the seven dictionaries of
// the lifecycle (the reason, the target roles, the ack-deadline policy,
// the return results, the event kinds, the lifecycle states, the client
// statuses), the read-time lifecycle derivation (the last event wins,
// the honest overdue derivation, the assignment-round material), the
// client status mapping («назначен ≠ принял» — two words), the
// conditional mode term of the unconditional ask (the 2.5 ratchet), the
// record guards of the six commands, the freeze gate of the new
// commitments (the byte-identical control branch) and the outbox
// perimeter (the identifiers and the dictionary words only).

// otherID is a second distinct identifier of the unit matrix (the
// newID helper is a constant — the «non-assignee» proofs need a
// different member).
func otherID() id.UUID {
	value, err := id.Parse("019a2f4c-1000-7000-8000-000000000002")
	if err != nil {
		panic(err)
	}
	return value
}

func handoffRequestOf() HandoffRequest {
	return HandoffRequest{
		TenantID: newID(), CaseID: newID(), RequestNo: 1,
		Reason:    HandoffReasonClientRequest,
		OwnerKind: OwnerKindStaffy, OwnerRef: newID(), OwnerEpoch: 1,
		TargetRoles: []string{HandoffTargetRoleOperator},
		AckDeadline: HandoffAck15m, AckDeadlineAt: time.Now().Add(15 * time.Minute),
		TurnRef: newID(), ActorID: newID(), CreatedAt: time.Now(),
	}
}

func handoffEventOf(request HandoffRequest, eventNo int64, kind string) HandoffEvent {
	return HandoffEvent{
		TenantID: request.TenantID, CaseID: request.CaseID, RequestNo: request.RequestNo,
		EventNo: eventNo, Kind: kind, Assignee: newID(), BackupOwner: newID(),
		AckDeadline: HandoffAck15m, DeadlineAt: time.Now().Add(15 * time.Minute),
		ActorID: newID(), CreatedAt: time.Now(),
	}
}

func TestHandoffDictionariesClosed(t *testing.T) {
	// The Story 4.4 extension: the reason dictionary carries FIVE words —
	// the unconditional client word of 2.13 plus the three operator
	// boundary words of the part request classification (the SQL mirror
	// is migrations/cases/00019, append-only by the 00018 precedent) plus
	// the fitment research word of the applicability panel (the SQL mirror
	// is migrations/cases/00021, append-only by the 00019 precedent).
	if len(HandoffReasons) != 5 || HandoffReasons[0] != HandoffReasonClientRequest {
		t.Fatalf("the reason dictionary carries the client word first plus the three Story 4.3 boundary words plus the Story 4.4 research word, got %v", HandoffReasons)
	}
	for _, reason := range []string{HandoffReasonDiagnosticUncertainty, HandoffReasonSafetyCritical, HandoffReasonUnresolvedContradiction} {
		if !HandoffReasonInDictionary(reason) {
			t.Fatalf("the Story 4.3 boundary word %q must be in the reason dictionary (the 00019 mirror)", reason)
		}
		if HandoffReasonText(reason) == "" {
			t.Fatalf("the Story 4.3 boundary word %q must carry its honest text", reason)
		}
	}
	if !HandoffReasonInDictionary(HandoffReasonFitmentResearch) {
		t.Fatalf("the Story 4.4 research word %q must be in the reason dictionary (the 00021 mirror)", HandoffReasonFitmentResearch)
	}
	if HandoffReasonText(HandoffReasonFitmentResearch) == "" {
		t.Fatalf("the Story 4.4 research word %q must carry its honest text", HandoffReasonFitmentResearch)
	}
	if HandoffReasonInDictionary("operator_asked") || !HandoffReasonInDictionary(HandoffReasonClientRequest) {
		t.Fatalf("the reason dictionary is closed on the five dictionary words")
	}
	if HandoffReasonText(HandoffReasonClientRequest) == "" || HandoffReasonText("made_up") != "" {
		t.Fatalf("the reason word carries its honest text, an unknown word renders none")
	}
	if len(HandoffTargetRoles) != 2 {
		t.Fatalf("the target role dictionary carries the two words, got %d", len(HandoffTargetRoles))
	}
	for _, role := range HandoffTargetRoles {
		if !HandoffTargetRoleInDictionary(role) {
			t.Fatalf("the role word %q is not in the dictionary", role)
		}
	}
	if HandoffTargetRoleInDictionary("billing") {
		t.Fatalf("an invented role never enters the dictionary")
	}
	if len(HandoffAckDeadlines) != 3 {
		t.Fatalf("the ack-deadline dictionary carries the three policy words, got %d", len(HandoffAckDeadlines))
	}
	for _, word := range HandoffAckDeadlines {
		if !HandoffAckDeadlineInDictionary(word) {
			t.Fatalf("the deadline word %q is not in the dictionary", word)
		}
		if HandoffAckDeadlineText(word) == "" {
			t.Fatalf("every deadline word carries its honest text, %q does not", word)
		}
		if HandoffAckDeadlineWindow(word) <= 0 {
			t.Fatalf("every deadline word carries its window, %q does not", word)
		}
	}
	if HandoffAckDeadlineInDictionary("ack_8h") || HandoffAckDeadlineWindow("ack_8h") != 0 {
		t.Fatalf("a word outside the approved policy never carries a window (the deadline is never bred)")
	}
	if len(HandoffResultWords) != 2 {
		t.Fatalf("the return result dictionary carries the two words, got %d", len(HandoffResultWords))
	}
	for _, word := range HandoffResultWords {
		if !HandoffResultInDictionary(word) {
			t.Fatalf("the result word %q is not in the dictionary", word)
		}
		if HandoffResultText(word) == "" {
			t.Fatalf("every result word carries its honest text, %q does not", word)
		}
	}
	if HandoffResultInDictionary("failed") {
		t.Fatalf("the failed word never exists in this slice (a word without a material event is a lie)")
	}
	if len(HandoffEventKinds) != 5 {
		t.Fatalf("the event kind dictionary carries the five lifecycle words, got %d", len(HandoffEventKinds))
	}
	for _, kind := range HandoffEventKinds {
		if !HandoffEventKindInDictionary(kind) {
			t.Fatalf("the event kind %q is not in the dictionary", kind)
		}
	}
	if HandoffEventKindInDictionary("failed") {
		t.Fatalf("the failed event kind never exists in this slice")
	}
	if len(HandoffStates) != 7 {
		t.Fatalf("the lifecycle state dictionary carries the seven words, got %d", len(HandoffStates))
	}
	for _, word := range HandoffStates {
		if !HandoffStateInDictionary(word) {
			t.Fatalf("the state word %q is not in the dictionary", word)
		}
		if HandoffStateText(word) == "" {
			t.Fatalf("every state word carries its honest text, %q does not", word)
		}
	}
	if len(HandoffClientStatuses) != 4 {
		t.Fatalf("the client status dictionary carries the four words, got %d", len(HandoffClientStatuses))
	}
	for _, word := range HandoffClientStatuses {
		if !HandoffClientStatusInDictionary(word) {
			t.Fatalf("the client status word %q is not in the dictionary", word)
		}
		if HandoffClientStatusText(word) == "" {
			t.Fatalf("every client status word carries its honest text, %q does not", word)
		}
	}
	if HandoffClientStatusInDictionary("human_connected") {
		t.Fatalf("an invented client status never enters the dictionary")
	}
}

// TestGuardHandoffRequestReason pins the Story 4.3 reason guard of the
// request creation: the EMPTY word is the unconditional client ask of
// 2.13 (never an input); the three operator boundary words pass; an
// explicit client_request input is the honest refusal (the word is the
// system truth); every other word is the honest refusal with the
// dictionary list.
func TestGuardHandoffRequestReason(t *testing.T) {
	if err := GuardHandoffRequestReason(""); err != nil {
		t.Fatalf("the empty word is the unconditional client ask — never an error, got %v", err)
	}
	for _, reason := range []string{HandoffReasonDiagnosticUncertainty, HandoffReasonSafetyCritical, HandoffReasonUnresolvedContradiction} {
		if err := GuardHandoffRequestReason(reason); err != nil {
			t.Fatalf("the operator boundary word %q must pass the guard, got %v", reason, err)
		}
	}
	if err := GuardHandoffRequestReason(HandoffReasonClientRequest); err == nil {
		t.Fatal("an explicit client_request input must be refused — the word is the system truth, never typed")
	}
	err := GuardHandoffRequestReason("operator_asked")
	if err == nil {
		t.Fatal("an out-of-dictionary word must be refused")
	}
	for _, reason := range []string{HandoffReasonDiagnosticUncertainty, HandoffReasonSafetyCritical, HandoffReasonUnresolvedContradiction} {
		if !strings.Contains(err.Error(), reason) {
			t.Fatalf("the refusal must carry the honest dictionary list including %q, got %v", reason, err)
		}
	}
}

func TestHumanRequestReasonConstantBound(t *testing.T) {
	if HumanRequestReasonText == "" {
		t.Fatalf("the system reason of the unconditional ask is never empty")
	}
	if len([]rune(HumanRequestReasonText)) > MaxModeReasonRunes {
		t.Fatalf("the system reason respects the mode reason rune bound: %d runes", len([]rune(HumanRequestReasonText)))
	}
}

func TestDeriveHandoffStateLifecycle(t *testing.T) {
	request := handoffRequestOf()
	now := time.Now()
	// No events: the ask waits for the assignment (an OPEN lifecycle).
	if view := DeriveHandoffState(request, nil, now); view.State != HandoffStateRequested || !view.Active() {
		t.Fatalf("the fresh ask derives requested and is active, got %q active=%v", view.State, view.Active())
	}
	// The assigned round before the deadline: assigned.
	assigned := handoffEventOf(request, 1, HandoffEventAssigned)
	if view := DeriveHandoffState(request, []HandoffEvent{assigned}, now); view.State != HandoffStateAssigned || !view.Active() {
		t.Fatalf("the open round derives assigned, got %q active=%v", view.State, view.Active())
	}
	// The same round after the deadline: overdue — the read-time
	// derivation, never a stored status.
	if view := DeriveHandoffState(request, []HandoffEvent{assigned}, assigned.DeadlineAt.Add(time.Minute)); view.State != HandoffStateOverdue || !view.Active() {
		t.Fatalf("the expired round derives overdue, got %q active=%v", view.State, view.Active())
	}
	if view := DeriveHandoffState(request, []HandoffEvent{assigned}, now); view.Assignee != assigned.Assignee || view.BackupOwner != assigned.BackupOwner {
		t.Fatalf("the round material travels with the derived state (the assignee and the backup)")
	}
	// The acknowledged acceptance closes the lifecycle (the only
	// transition that moves the ownership — together with returned).
	acknowledged := handoffEventOf(request, 2, HandoffEventAcknowledged)
	acknowledged.ResultingEpoch = 7
	if view := DeriveHandoffState(request, []HandoffEvent{assigned, acknowledged}, now); view.State != HandoffStateAcknowledged || view.Active() || view.ResultingEpoch != 7 {
		t.Fatalf("the acceptance closes the lifecycle and carries the resulting epoch, got %q active=%v epoch=%d", view.State, view.Active(), view.ResultingEpoch)
	}
	// The decline consumes the round and keeps the ask open.
	declined := handoffEventOf(request, 2, HandoffEventDeclined)
	if view := DeriveHandoffState(request, []HandoffEvent{assigned, declined}, now); view.State != HandoffStateDeclined || !view.Active() {
		t.Fatalf("the refusal keeps the ask open, got %q active=%v", view.State, view.Active())
	}
	// The re-assignment after the decline: a NEW round with a NEW
	// deadline («повторный assign — новое событие с новым дедлайном»).
	reassigned := handoffEventOf(request, 3, HandoffEventAssigned)
	reassigned.DeadlineAt = now.Add(time.Hour)
	if view := DeriveHandoffState(request, []HandoffEvent{assigned, declined, reassigned}, now); view.State != HandoffStateAssigned || view.DeadlineAt != reassigned.DeadlineAt {
		t.Fatalf("the re-assigned round derives assigned with its own deadline, got %q", view.State)
	}
	// The cancelled ask closes (the client's cancellation).
	cancelled := handoffEventOf(request, 2, HandoffEventCancelled)
	if view := DeriveHandoffState(request, []HandoffEvent{cancelled}, now); view.State != HandoffStateCancelled || view.Active() {
		t.Fatalf("the cancellation closes the lifecycle, got %q active=%v", view.State, view.Active())
	}
	// The return closes with the result word.
	returned := handoffEventOf(request, 3, HandoffEventReturned)
	returned.Result = HandoffResultIssueResolved
	if view := DeriveHandoffState(request, []HandoffEvent{assigned, acknowledged, returned}, now); view.State != HandoffStateReturned || view.Active() || view.Result != HandoffResultIssueResolved {
		t.Fatalf("the return closes the lifecycle with its result word, got %q result=%q", view.State, view.Result)
	}
	// The LAST event wins on any input order (the append-only history is
	// never rewritten; the equal-number tie is the defensive backstop).
	first := handoffEventOf(request, 1, HandoffEventAssigned)
	second := handoffEventOf(request, 2, HandoffEventDeclined)
	if view := DeriveHandoffState(request, []HandoffEvent{second, first}, now); view.State != HandoffStateDeclined {
		t.Fatalf("the maximal event number decides the state regardless of the read order, got %q", view.State)
	}
	// The events of ANOTHER request never leak into this lifecycle.
	foreign := handoffEventOf(request, 1, HandoffEventCancelled)
	foreign.RequestNo = 2
	if view := DeriveHandoffState(request, []HandoffEvent{foreign}, now); view.State != HandoffStateRequested {
		t.Fatalf("a foreign request's events never derive this lifecycle, got %q", view.State)
	}
}

func TestDeriveHandoffClientStatusMapping(t *testing.T) {
	cases := map[string]string{
		HandoffStateRequested:    HandoffClientRequested,
		HandoffStateDeclined:     HandoffClientRequested,
		HandoffStateAssigned:     HandoffClientAssigned,
		HandoffStateOverdue:      HandoffClientAssigned,
		HandoffStateAcknowledged: HandoffClientAcknowledged,
		HandoffStateReturned:     HandoffClientAcknowledged,
		HandoffStateCancelled:    HandoffClientCancelled,
	}
	for state, expected := range cases {
		if got := DeriveHandoffClientStatus(state); got != expected {
			t.Fatalf("the client status of %q is %q, got %q", state, expected, got)
		}
	}
	// «Назначен ≠ принял» — TWO different words, never one.
	if DeriveHandoffClientStatus(HandoffStateAssigned) == DeriveHandoffClientStatus(HandoffStateAcknowledged) {
		t.Fatalf("the assignment and the acceptance are two different client words (AC-11)")
	}
	// The interface never claims the specialist is connected before the
	// confirmed acceptance.
	if DeriveHandoffClientStatus(HandoffStateAssigned) != HandoffClientAssigned {
		t.Fatalf("the assigned round stays the assigned word until the acceptance")
	}
}

func TestHandoffModeTermRatchet(t *testing.T) {
	tenantID, caseID, conversationID := newID(), newID(), newID()
	turnRef, actorID := newID(), newID()
	now := time.Now()
	// The absent ladder raises from the admission floor.
	term, ok := HandoffModeTerm(nil, tenantID, caseID, conversationID, turnRef, actorID, now)
	if !ok || term == nil {
		t.Fatalf("the absent ladder raises to human_requested")
	}
	if term.Mode != ModeHumanRequested || term.Scope != ModeScopeConversation || term.ModeVersion != 1 {
		t.Fatalf("the raise carries the mode word, the conversation scope and the first version, got %q %q %d", term.Mode, term.Scope, term.ModeVersion)
	}
	if term.SignalSource != ModeSignalTurnEvent || term.TurnRef != turnRef {
		t.Fatalf("the raise anchors the asking turn (the turn_event signal source)")
	}
	if term.Reason != HumanRequestReasonText || term.Origin != ModeOriginProposal {
		t.Fatalf("the raise carries the system reason constant through the proposal origin")
	}
	if term.PriorMode != ModeUnclassified {
		t.Fatalf("the absent ladder starts from unclassified, got %q", term.PriorMode)
	}
	// The sales ladder raises (the ratchet direction).
	sales := &ModeTerm{TenantID: tenantID, Mode: ModeSales, ModeVersion: 4}
	term, ok = HandoffModeTerm(sales, tenantID, caseID, conversationID, turnRef, actorID, now)
	if !ok || term.ModeVersion != 5 || term.PriorMode != ModeSales {
		t.Fatalf("the sales raise carries the CAS version and the prior word, got version=%d prior=%q", term.ModeVersion, term.PriorMode)
	}
	// The equal mode is NEVER touched (the ask is unconditional, the
	// handoff row is still created — the ladder keeps its word).
	equal := &ModeTerm{Mode: ModeHumanRequested, ModeVersion: 4}
	if term, ok := HandoffModeTerm(equal, tenantID, caseID, conversationID, turnRef, actorID, now); ok || term != nil {
		t.Fatalf("the equal human_requested mode is never touched by the repeated ask")
	}
	// A STRICTER mode is never touched (the ratchet: the ask does not
	// lower safety/claim/cancellation/privacy).
	for _, stricter := range []string{ModeSafety, ModeClaim, ModeCancellation, ModePrivacyNoContact} {
		active := &ModeTerm{Mode: stricter, ModeVersion: 4}
		if term, ok := HandoffModeTerm(active, tenantID, caseID, conversationID, turnRef, actorID, now); ok || term != nil {
			t.Fatalf("the stricter mode %q is never touched by the ask", stricter)
		}
	}
}

func TestGuardRequestHuman(t *testing.T) {
	// The honest material passes.
	if err := GuardRequestHuman(newID(), 1, []string{HandoffTargetRoleOperator}, HandoffAck15m); err != nil {
		t.Fatalf("the honest ask material passes, got %v", err)
	}
	if err := GuardRequestHuman(newID(), 2, HandoffTargetRoles, HandoffAck4h); err != nil {
		t.Fatalf("the full role subset passes, got %v", err)
	}
	// The turn anchor is mandatory.
	if err := GuardRequestHuman(id.UUID(""), 1, []string{HandoffTargetRoleOperator}, HandoffAck15m); err == nil {
		t.Fatalf("the ask without the turn anchor refuses")
	}
	// The CAS expectation is mandatory.
	if err := GuardRequestHuman(newID(), 0, []string{HandoffTargetRoleOperator}, HandoffAck15m); err == nil {
		t.Fatalf("the ask without the mode version expectation refuses")
	}
	// The roles are a non-empty subset without duplicates.
	if err := GuardRequestHuman(newID(), 1, nil, HandoffAck15m); err == nil {
		t.Fatalf("the ask without the target roles refuses")
	}
	if err := GuardRequestHuman(newID(), 1, []string{"billing"}, HandoffAck15m); err == nil {
		t.Fatalf("a role outside the dictionary refuses")
	}
	if err := GuardRequestHuman(newID(), 1, []string{HandoffTargetRoleOperator, HandoffTargetRoleOperator}, HandoffAck15m); err == nil {
		t.Fatalf("a duplicated role refuses")
	}
	// The deadline word belongs to the approved policy.
	if err := GuardRequestHuman(newID(), 1, []string{HandoffTargetRoleOperator}, "ack_8h"); err == nil {
		t.Fatalf("a deadline word outside the policy refuses")
	}
}

func TestGuardAssignHandoff(t *testing.T) {
	targetRoles := []string{HandoffTargetRoleOperator, HandoffTargetRoleSupervision}
	assignee, backup := newID(), newID()
	// The open states pass: the fresh ask, the expired round, the refused
	// round (the human re-assignment).
	for _, state := range []string{HandoffStateRequested, HandoffStateOverdue, HandoffStateDeclined} {
		if err := GuardAssignHandoff(state, assignee, HandoffTargetRoleOperator, targetRoles, backup, HandoffAck1h); err != nil {
			t.Fatalf("the assignment from %q passes, got %v", state, err)
		}
	}
	// The closed lifecycles refuse.
	for _, state := range []string{HandoffStateAcknowledged, HandoffStateCancelled, HandoffStateReturned} {
		if err := GuardAssignHandoff(state, assignee, HandoffTargetRoleOperator, targetRoles, backup, HandoffAck1h); err == nil {
			t.Fatalf("the assignment from the closed state %q refuses", state)
		}
	}
	// The assignee and the backup are the member references.
	if err := GuardAssignHandoff(HandoffStateRequested, id.UUID(""), HandoffTargetRoleOperator, targetRoles, backup, HandoffAck1h); err == nil {
		t.Fatalf("the assignment without the assignee refuses")
	}
	if err := GuardAssignHandoff(HandoffStateRequested, assignee, HandoffTargetRoleOperator, targetRoles, id.UUID(""), HandoffAck1h); err == nil {
		t.Fatalf("the assignment without the backup refuses")
	}
	// The assignee's role belongs to the request's target roles.
	if err := GuardAssignHandoff(HandoffStateRequested, assignee, "billing", targetRoles, backup, HandoffAck1h); err == nil {
		t.Fatalf("a role outside the dictionary refuses")
	}
	if err := GuardAssignHandoff(HandoffStateRequested, assignee, HandoffTargetRoleSupervision, []string{HandoffTargetRoleOperator}, backup, HandoffAck1h); err == nil {
		t.Fatalf("a role outside the request's target roles refuses")
	}
	// The round's deadline word belongs to the approved policy.
	if err := GuardAssignHandoff(HandoffStateRequested, assignee, HandoffTargetRoleOperator, targetRoles, backup, "tomorrow"); err == nil {
		t.Fatalf("a deadline word outside the policy refuses")
	}
}

func TestGuardHandoffAcknowledge(t *testing.T) {
	assignee := newID()
	// The open assigned round passes with the assignee as the actor.
	if err := GuardHandoffAcknowledge(HandoffStateAssigned, assignee, assignee, 3); err != nil {
		t.Fatalf("the acceptance from the assigned round passes, got %v", err)
	}
	// The expired round refuses: the deadline closed the round honestly.
	if err := GuardHandoffAcknowledge(HandoffStateOverdue, assignee, assignee, 3); err == nil {
		t.Fatalf("the acceptance from the expired round refuses (the re-assignment is the honest path)")
	}
	// The closed or fresh states refuse.
	for _, state := range []string{HandoffStateRequested, HandoffStateAcknowledged, HandoffStateDeclined, HandoffStateCancelled, HandoffStateReturned} {
		if err := GuardHandoffAcknowledge(state, assignee, assignee, 3); err == nil {
			t.Fatalf("the acceptance from %q refuses", state)
		}
	}
	// Only the assignee confirms (the server truth — the assignment or
	// the notification is never an acknowledgement).
	if err := GuardHandoffAcknowledge(HandoffStateAssigned, otherID(), assignee, 3); err == nil {
		t.Fatalf("a non-assignee actor never confirms the acceptance")
	}
	// The CAS epoch expectation is mandatory.
	if err := GuardHandoffAcknowledge(HandoffStateAssigned, assignee, assignee, 0); err == nil {
		t.Fatalf("the acceptance without the epoch expectation refuses")
	}
}

func TestGuardHandoffDecline(t *testing.T) {
	assignee := newID()
	if err := GuardHandoffDecline(HandoffStateAssigned, assignee, assignee); err != nil {
		t.Fatalf("the refusal from the assigned round passes, got %v", err)
	}
	if err := GuardHandoffDecline(HandoffStateOverdue, assignee, assignee); err == nil {
		t.Fatalf("the refusal from the expired round refuses (the deadline already closed the round)")
	}
	if err := GuardHandoffDecline(HandoffStateAssigned, otherID(), assignee); err == nil {
		t.Fatalf("a non-assignee actor never refuses the round")
	}
	if err := GuardHandoffDecline(HandoffStateAcknowledged, assignee, assignee); err == nil {
		t.Fatalf("the refusal from the closed lifecycle refuses")
	}
}

func TestGuardCancelHumanRequest(t *testing.T) {
	// The open lifecycle passes with the cancelling turn anchor.
	for _, state := range []string{HandoffStateRequested, HandoffStateAssigned, HandoffStateOverdue, HandoffStateDeclined} {
		if err := GuardCancelHumanRequest(state, newID()); err != nil {
			t.Fatalf("the cancellation from the open %q passes, got %v", state, err)
		}
	}
	// The closed lifecycle refuses (the acknowledged case returns
	// through return_to_staffy).
	for _, state := range []string{HandoffStateAcknowledged, HandoffStateCancelled, HandoffStateReturned} {
		if err := GuardCancelHumanRequest(state, newID()); err == nil {
			t.Fatalf("the cancellation from the closed %q refuses", state)
		}
	}
	// The cancelling turn anchor is mandatory.
	if err := GuardCancelHumanRequest(HandoffStateAssigned, id.UUID("")); err == nil {
		t.Fatalf("the cancellation without the turn anchor refuses")
	}
}

func TestGuardReturnToStaffy(t *testing.T) {
	if err := GuardReturnToStaffy(HandoffStateAcknowledged, HandoffResultIssueResolved); err != nil {
		t.Fatalf("the return from the accepted lifecycle passes, got %v", err)
	}
	if err := GuardReturnToStaffy(HandoffStateAcknowledged, HandoffResultHandedBack); err != nil {
		t.Fatalf("both result words pass, got %v", err)
	}
	if err := GuardReturnToStaffy(HandoffStateAssigned, HandoffResultIssueResolved); err == nil {
		t.Fatalf("the return from the unaccepted round refuses")
	}
	if err := GuardReturnToStaffy(HandoffStateAcknowledged, "failed"); err == nil {
		t.Fatalf("a result word outside the dictionary refuses (the failed word never exists)")
	}
}

func TestGuardCommitmentFreeze(t *testing.T) {
	// The control branch: no mode and no handoff — the 2.9 path is
	// byte-identical (nil, never a side effect).
	if err := GuardCommitmentFreeze(nil, false); err != nil {
		t.Fatalf("the control branch without the mode and the handoff stays byte-identical, got %v", err)
	}
	if err := GuardCommitmentFreeze(&ModeTerm{Mode: ModeSales}, false); err != nil {
		t.Fatalf("the sales mode never freezes the commitments of its own word, got %v", err)
	}
	// The freeze: the effective mode EQUALS human_requested.
	err := GuardCommitmentFreeze(&ModeTerm{Mode: ModeHumanRequested}, false)
	if err == nil {
		t.Fatalf("the human_requested mode freezes the new commitments")
	}
	if !strings.Contains(err.Error(), "human_requested") {
		t.Fatalf("the freeze refusal carries the mode word, got %v", err)
	}
	// The freeze: the active handoff freezes under ANY mode (the ask
	// exists — even a stricter ladder word).
	if err := GuardCommitmentFreeze(nil, true); err == nil {
		t.Fatalf("the active handoff freezes the new commitments")
	}
	if err := GuardCommitmentFreeze(&ModeTerm{Mode: ModeSafety}, true); err == nil {
		t.Fatalf("the active handoff freezes under the stricter mode too")
	}
	// A stricter mode WITHOUT an active handoff is not this gate's word
	// (the mode ladder of 2.5 governs its own paths).
	if err := GuardCommitmentFreeze(&ModeTerm{Mode: ModeSafety}, false); err != nil {
		t.Fatalf("the stricter mode without the handoff never fires this gate, got %v", err)
	}
	if err := GuardCommitmentFreeze(&ModeTerm{Mode: ModeUnclassified}, false); err != nil {
		t.Fatalf("the unclassified mode never fires this gate, got %v", err)
	}
}

func TestHandoffEventPayloadPerimeter(t *testing.T) {
	request := handoffRequestOf()
	event := handoffEventOf(request, 2, HandoffEventAcknowledged)
	event.ResultingEpoch = 5
	payload := HandoffEventPayload{
		TenantID: request.TenantID, CaseID: request.CaseID,
		RequestNo: request.RequestNo, EventNo: event.EventNo, Kind: event.Kind,
		Reason: request.Reason, TargetRoles: request.TargetRoles,
		AckDeadline: event.AckDeadline, DeadlineAt: event.DeadlineAt,
		Assignee: event.Assignee, BackupOwner: event.BackupOwner,
		Result: event.Result, ResultingEpoch: event.ResultingEpoch,
		TurnRef: request.TurnRef, ActorID: event.ActorID, OccurredAt: event.CreatedAt,
	}
	encoded, err := HandoffEventPayloadJSON(payload)
	if err != nil {
		t.Fatalf("the payload renders its canonical JSON, got %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("the canonical JSON decodes, got %v", err)
	}
	for _, field := range []string{"tenant_id", "case_id", "request_no", "event_no", "kind", "reason", "target_roles", "ack_deadline", "deadline_at", "assignee", "backup_owner", "result", "resulting_epoch", "turn_ref", "actor_id", "occurred_at"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("the payload carries the field %q", field)
		}
	}
	// The system reason text of the MODE never enters the envelope (the
	// request reason is the dictionary word, never the mode text).
	if string(encoded) != "" && strings.Contains(string(encoded), HumanRequestReasonText) {
		t.Fatalf("the mode reason text never enters the outbox perimeter")
	}
	// The dedup keys of two events differ.
	first := DedupKeyHandoffEvent(request.CaseID, request.RequestNo, 1)
	second := DedupKeyHandoffEvent(request.CaseID, request.RequestNo, 2)
	if first == second || !strings.HasPrefix(first, "case.human_handoff_event:") {
		t.Fatalf("the dedup key is the per-event identity, got %q and %q", first, second)
	}
	// The six outbox event types of the lifecycle.
	words := []string{OutboxEventHumanRequested, OutboxEventHumanAssigned, OutboxEventHumanAcknowledged, OutboxEventHumanDeclined, OutboxEventHumanCancelled, OutboxEventHumanReturned}
	seen := map[string]bool{}
	for _, word := range words {
		if seen[word] {
			t.Fatalf("the outbox event type %q repeats", word)
		}
		seen[word] = true
	}
}
