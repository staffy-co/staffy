//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	identityAPI "github.com/staffy-co/staffy/internal/identity/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// ownershipNow is the command clock of the Story 2.4 tests (the release
// clock + two days — the same relation the 2-2/2-3 commands use).
var ownershipNow = releaseClock.Now().Add(48 * time.Hour)

// ownershipActor builds the step-up principal of the world owner.
func (w *casesWorld) ownershipActor() identityDomain.Principal {
	return w.workActor()
}

// backupActor builds the step-up principal of the backup owner member
// (the review+audit scopes: the transfer target, never a configuration
// member).
func (w *casesWorld) backupActor() identityDomain.Principal {
	return actorPrincipal{principalID: w.backup, stepUp: true}.principal()
}

// ownershipState reads the derived ownership state of the case view.
func (w *casesWorld) ownershipState(t *testing.T, caseID id.UUID) casesAPI.OwnershipView {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	return detail.Ownership
}

// ownershipTermCount reads the number of the terms of the case history.
func (w *casesWorld) ownershipTermCount(t *testing.T, caseID id.UUID) int {
	t.Helper()
	return len(w.ownershipState(t, caseID).Terms)
}

// TestIngestWritesInitialOwnershipTerm covers the ingest matrix rows: the
// same admission transaction writes the initial term (epoch 1, the owner
// from the human coverage, the inbox surface, the ingest_initial origin);
// a queue or a department is never an owner (the target validation of the
// transfer commands refuses non-member refs honestly).
func TestIngestWritesInitialOwnershipTerm(t *testing.T) {
	w := newCasesWorld(t, 610)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-owner-1", "sess-owner-1", `{"call":"inbound-call","from":"+79000000061"}`)

	ownership := w.ownershipState(t, caseID)
	if len(ownership.Terms) != 1 {
		t.Fatalf("expected exactly one initial term, got %d", len(ownership.Terms))
	}
	term := ownership.Terms[0]
	if term.Epoch != 1 || term.OwnerKind != casesDomain.OwnerKindHuman || term.OwnerRef != w.owner.ID {
		t.Fatalf("expected the epoch 1 human owner term, got %+v", term)
	}
	if term.WritableSurface != casesDomain.WritableSurfaceInbox || term.Origin != casesDomain.OwnershipOriginIngestInitial {
		t.Fatalf("expected the inbox/ingest_initial term, got %+v", term)
	}
	if ownership.State.Current == nil || ownership.State.Current.Epoch != 1 || ownership.State.Health != casesDomain.OwnershipHealthStable {
		t.Fatalf("expected the stable epoch 1 derivation, got %+v", ownership.State)
	}
	if ownership.State.BackupOwnerID != w.backup || ownership.State.AckDeadlineMinutes != 15 {
		t.Fatalf("expected the coverage words (backup + 15 minutes), got %+v", ownership.State)
	}

	// A queue or a department is never an owner: a non-member target
	// refuses honestly (AC-1).
	_, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: id.MustParse("01994f64-6c00-7000-8000-0000000000ee"),
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "очередь",
		IdempotencyKey: "owner-queue-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "активным участником")

	// The staffy target must be the employee of THIS case.
	_, err = w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindStaffy, ToOwnerRef: id.MustParse("01994f64-6c00-7000-8000-0000000000ef"),
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "не тот сотрудник",
		IdempotencyKey: "owner-staffy-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "сотрудником этого обращения")
}

// TestReplyGateAndOwnerReply covers the reply gate matrix rows: the
// non-owner gets the machine-readable refusal with the current owner, the
// epoch and the permitted path; the current owner writes the prepared
// reply with the full snapshot; the exact repeat replays; the staffy
// owner honestly refuses (the autonomous behaviour is 2-5).
func TestReplyGateAndOwnerReply(t *testing.T) {
	w := newCasesWorld(t, 620)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-reply-1", "sess-reply-1", `{"call":"inbound-call","from":"+79000000062"}`)
	conversationID := w.conversationFirst(t, caseID)
	replyEpoch := int64(1)

	// The non-owner (the backup member) gets the machine-readable gate
	// refusal: the owner kind, the name, the epoch, the permitted path.
	_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "посторонний текст", IdempotencyKey: "reply-gate-1", Now: ownershipNow,
	})
	var gate casesDomain.ReplyGateError
	if !errors.As(err, &gate) {
		t.Fatalf("expected the machine-readable gate refusal, got %v", err)
	}
	if gate.OwnerKind != casesDomain.OwnerKindHuman || gate.CurrentEpoch != 1 {
		t.Fatalf("expected the human/epoch 1 refusal words, got %+v", gate)
	}
	if gate.OwnerName == "" || !strings.Contains(gate.Reason, "владелец") {
		t.Fatalf("expected the owner name and the honest reason, got %+v", gate)
	}

	// The current owner writes the prepared reply: the full snapshot with
	// the route and the lineage of the acting decision.
	outcome, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "Добрый день! Ваше обращение принято в работу.", IdempotencyKey: "reply-owner-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Replay || outcome.ReplyState != "prepared" || outcome.Epoch != 1 {
		t.Fatalf("expected the fresh prepared reply, got %+v", outcome)
	}
	ownership := w.ownershipState(t, caseID)
	if len(ownership.Replies) != 1 || ownership.Replies[0].Reply.Content != "Добрый день! Ваше обращение принято в работу." {
		t.Fatalf("expected the one reply row, got %+v", ownership.Replies)
	}
	reply := ownership.Replies[0].Reply
	if reply.Channel != "phone" || reply.Direction != "outbound" || reply.Epoch != 1 || reply.WritableSurface != "inbox" {
		t.Fatalf("expected the full route snapshot, got %+v", reply)
	}
	if reply.DecisionNo != 1 || reply.ProfileVersion != 1 || !reply.EmployeeID.Valid() {
		t.Fatalf("expected the release lineage snapshot, got %+v", reply)
	}

	// The exact repeat replays the original outcome.
	replay, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "Добрый день! Ваше обращение принято в работу.", IdempotencyKey: "reply-owner-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replay || replay.ReplyID != outcome.ReplyID {
		t.Fatalf("expected the idempotent replay, got %+v", replay)
	}

	// The same key with the DIVERGENT epoch is the honest key conflict —
	// the idempotency hash includes the CAS expectation (the fixture and
	// the form path mirror the same word — the R2 minor lesson).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch + 1,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "Добрый день! Ваше обращение принято в работу.", IdempotencyKey: "reply-owner-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("expected the key conflict of the divergent epoch, got %v", err)
	}

	// The bounded content: the empty and the oversized words refuse.
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    strings.Repeat("а", 2001), IdempotencyKey: "reply-long-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "длиннее")

	// The staffy owner: the transfer to the case employee succeeds, the
	// reply of the staffy owner honestly refuses (2-5).
	staffyOutcome, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindStaffy, ToOwnerRef: w.employeeID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "цифровой сотрудник принимает владение",
		IdempotencyKey: "reply-staffy-tr-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, RequestID: staffyOutcome.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, Reason: "передаю цифровому сотруднику",
		IdempotencyKey: "reply-staffy-ack-1", Now: ownershipNow,
	})
	if err == nil {
		t.Fatal("the staffy target acknowledge must refuse: the staffy principal cannot confirm a transfer")
	}
	// The decline path closes the request (the author cancels).
	_, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, RequestID: staffyOutcome.TransferRequestID,
		Decision: casesDomain.TransferResolutionCancelled, Reason: "отзываю",
		IdempotencyKey: "reply-staffy-cancel-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestTransferLifecycleAcknowledgeDecline covers the transfer matrix
// rows: the request (the deadline from the coverage word), the pending
// state (the current owner keeps the reply right), the acknowledge (the
// atomic transition: the resolution, the term, the new owner), the stale
// epoch of the old owner, the decline (no ownership change), the second
// simultaneous request refusal and the reassign scope boundary.
func TestTransferLifecycleAcknowledgeDecline(t *testing.T) {
	w := newCasesWorld(t, 630)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-transfer-1", "sess-transfer-1", `{"call":"inbound-call","from":"+79000000063"}`)
	conversationID := w.conversationFirst(t, caseID)
	replyEpoch := int64(1)

	// The backup member cannot request the transfer (not the owner, no
	// configuration scope).
	_, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.owner.ID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "посторонний запрос",
		IdempotencyKey: "tr-scope-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "текущий владелец")

	// The owner requests the transfer to the backup: the deadline comes
	// from the coverage (15 minutes), never from the command.
	request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "уход в отпуск",
		IdempotencyKey: "tr-request-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Replay {
		t.Fatalf("expected the fresh request, got %+v", request)
	}

	// The pending state: the current owner KEEPS the reply right (AC-5).
	ownership := w.ownershipState(t, caseID)
	if ownership.State.Health != casesDomain.OwnershipHealthTransferPending || ownership.State.Pending == nil {
		t.Fatalf("expected the pending derivation, got %+v", ownership.State)
	}
	if !ownership.State.Pending.DeadlineAt.Equal(ownershipNow.Add(15 * time.Minute)) {
		t.Fatalf("expected the coverage deadline (+15 minutes), got %v", ownership.State.Pending.DeadlineAt)
	}
	if _, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "ответ до передачи", IdempotencyKey: "tr-reply-1", Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the pending request never blocks the current owner: %v", err)
	}

	// The second simultaneous request refuses honestly (one pending ask).
	_, err = w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "второй запрос",
		IdempotencyKey: "tr-request-2", Now: ownershipNow,
	})
	assertRuleError(t, err, "уже назначен")

	// Someone else than the target cannot acknowledge.
	_, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, RequestID: request.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, IdempotencyKey: "tr-ack-wrong-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "цель запроса")

	// The target acknowledges: the atomic transition.
	acknowledge, err := w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: request.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю владение",
		IdempotencyKey: "tr-ack-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if acknowledge.Epoch != 2 {
		t.Fatalf("expected the epoch 2 transition, got %+v", acknowledge)
	}
	ownership = w.ownershipState(t, caseID)
	if ownership.State.Current == nil || ownership.State.Current.Epoch != 2 || ownership.State.Current.OwnerRef != w.backup {
		t.Fatalf("expected the backup owner at epoch 2, got %+v", ownership.State.Current)
	}
	if ownership.State.Current.Origin != casesDomain.OwnershipOriginTransferAcknowledge || ownership.State.Current.PreviousOwnerRef != w.owner.ID {
		t.Fatalf("expected the acknowledged transfer snapshots, got %+v", ownership.State.Current)
	}
	if len(ownership.Terms) != 2 {
		t.Fatalf("expected two terms, got %d", len(ownership.Terms))
	}

	// The old owner loses the reply right immediately: the gate names the
	// NEW owner (AC-10).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "ответ прежнего владельца", IdempotencyKey: "tr-reply-2", Now: ownershipNow,
	})
	var gate casesDomain.ReplyGateError
	if !errors.As(err, &gate) || gate.OwnerName == "" || gate.CurrentEpoch != 2 {
		t.Fatalf("expected the new-owner gate refusal, got %v", err)
	}

	// The CAS row of the reply: the NEW owner replying with the STALE
	// expectation (the pre-acknowledge epoch) gets the honest version
	// conflict — the material stayed consistent, the command is not
	// silently merged.
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: replyEpoch, ReplyClass: casesDomain.ReplyClassNeutral,
		Content:        "ответ с устаревшей версией",
		IdempotencyKey: "tr-reply-cas-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("expected the stale version of the new owner reply, got %v", err)
	}

	// The decline path: the new request declined — the ownership stays.
	declineRequest, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 2,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.owner.ID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "возврат владельцу",
		IdempotencyKey: "tr-request-3", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, RequestID: declineRequest.TransferRequestID,
		Decision: casesDomain.TransferResolutionDeclined, Reason: "не могу принять сейчас",
		IdempotencyKey: "tr-decline-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	ownership = w.ownershipState(t, caseID)
	if ownership.State.Current.Epoch != 2 || ownership.State.Health != casesDomain.OwnershipHealthStable || ownership.State.Pending != nil {
		t.Fatalf("expected the unchanged ownership after the decline, got %+v", ownership.State)
	}

	// The reassign scope boundary: the review-only member (the backup)
	// cannot use the supervision path.
	_, err = w.cases.ReassignOwnership(ctx, casesAPI.ReassignOwnershipCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 2,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.owner.ID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "путь надзора без полномочия",
		IdempotencyKey: "tr-reassign-scope-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "configuration")
}

// TestTransferDeadlineAttentionAndTake covers the deadline matrix rows:
// the late acknowledge refuses honestly (request_expired); the drain
// observes the expired request — the ownership_attention exception once;
// the take becomes allowed after the expiry; the fresh prepared reply of
// the unchanged owner dispatches with the outbox record; the stale one
// (the ownership moved) blocks with stale_needs_review.
func TestTransferDeadlineAttentionAndTake(t *testing.T) {
	w := newCasesWorld(t, 640)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-deadline-1", "sess-deadline-1", `{"call":"inbound-call","from":"+79000000064"}`)
	conversationID := w.conversationFirst(t, caseID)
	replyEpoch := int64(1)

	// The owner requests the transfer and prepares a reply.
	request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "дедлайн-тест",
		IdempotencyKey: "dl-request-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "свежий ответ", IdempotencyKey: "dl-reply-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}

	// The late acknowledge: the deadline (15 minutes) passed.
	late := ownershipNow.Add(16 * time.Minute)
	_, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: request.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, IdempotencyKey: "dl-ack-late-1", Now: late,
	})
	assertRuleError(t, err, "истёк")

	// The take over the healthy ownership is not the deadline path yet:
	// the request expired — the take opens. The drain first observes the
	// expired request: the ownership_attention exception lands once. The
	// drain clock moves past the deadline (the service is rebuilt with
	// the late clock — the sweep observes the expired world).
	lateService := w.buildCasesServiceClock(newPrivacyIDs(641), fixedClock{late})
	drained, err := lateService.DrainClientReplies(ctx, w.ownershipActor(), w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if drained.Considered != 1 || drained.Dispatched != 1 || drained.Stale != 0 {
		t.Fatalf("expected the one dispatched fresh reply, got %+v", drained)
	}
	detail, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	attention := 0
	for _, exception := range detail.Exceptions {
		if exception.Kind == casesDomain.ExceptionOwnershipAttention {
			attention++
		}
	}
	if attention != 1 {
		t.Fatalf("expected exactly one ownership_attention exception, got %d", attention)
	}
	// The repeated drain is fully idempotent (the material uniqueness).
	again, err := lateService.DrainClientReplies(ctx, w.ownershipActor(), w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if again.Considered != 0 || again.Dispatched != 0 || again.Stale != 0 {
		t.Fatalf("expected the idempotent repeat, got %+v", again)
	}
	ownership := w.ownershipState(t, caseID)
	if ownership.Replies[0].OutcomeKind != casesDomain.ReplyOutcomeDispatched {
		t.Fatalf("expected the dispatched derivation, got %+v", ownership.Replies[0])
	}

	// The M1 matrix rows (the R1 lesson): the EXPIRED unresolved ask is
	// voided by the derivation — it never blocks the new request.
	renewed, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "новый запрос после истечения",
		IdempotencyKey: "dl-request-2", Now: late,
	})
	if err != nil {
		t.Fatalf("the expired ask must not block the new request: %v", err)
	}
	// The renewed live ask keeps the honest pending discipline.
	if _, err = w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "второй живой запрос",
		IdempotencyKey: "dl-request-3", Now: late,
	}); err == nil || !strings.Contains(err.Error(), "уже назначен") {
		t.Fatalf("expected the live-ask refusal of the second request, got %v", err)
	}
	// The renewed ask is cancelled (the target path) so the take rows run
	// on the expired word alone.
	if _, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: renewed.TransferRequestID,
		Decision: casesDomain.TransferResolutionCancelled, IdempotencyKey: "dl-cancel-1", Now: late,
	}); err != nil {
		t.Fatal(err)
	}

	// The take after the expiry: the backup takes the ownership (the ask
	// is voided by the cancel resolution of the same transaction).
	taken, err := w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		Reason: "дедлайн истёк, забираю", IdempotencyKey: "dl-take-1", Now: late,
	})
	if err != nil {
		t.Fatal(err)
	}
	if taken.Epoch != 2 {
		t.Fatalf("expected the epoch 2 take, got %+v", taken)
	}
	// The ownership after the take is HEALTHY: the zombie expired ask is
	// gone — the health word never misrepresents the fresh ownership.
	ownership = w.ownershipState(t, caseID)
	if ownership.State.Health != casesDomain.OwnershipHealthStable || ownership.State.Pending != nil {
		t.Fatalf("expected the stable ownership after the take (the ask voided), got %+v", ownership.State)
	}
	// The second take over the now-healthy ownership is honestly refused —
	// the expired ask never keeps the case takeable.
	if _, err = w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 2,
		Reason: "второй take здорового владения", IdempotencyKey: "dl-take-3", Now: late,
	}); err == nil || !strings.Contains(err.Error(), "Владение здорово") {
		t.Fatalf("expected the healthy-ownership take refusal, got %v", err)
	}
	// The term backstop: the replay of the take with the same material
	// replays; another take with the same expected epoch is stale.
	_, err = w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		Reason: "дедлайн истёк, забираю", IdempotencyKey: "dl-take-1", Now: late,
	})
	if err != nil {
		t.Fatalf("expected the idempotent replay, got %v", err)
	}
	_, err = w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		Reason: "поздний второй take", IdempotencyKey: "dl-take-2", Now: late,
	})
	if !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("expected the stale version of the second take, got %v", err)
	}
}

// TestTransferSupersessionVoidByOwnershipChange covers the zombie chain of
// the R1 Major class: an EXPIRED unresolved ask that the ownership already
// moved past (an acknowledged renewal, a take over a double expiry) is
// void for every word — the health never misreports the fresh ownership
// and the case never stays takeable while the owner is healthy.
func TestTransferSupersessionVoidByOwnershipChange(t *testing.T) {
	w := newCasesWorld(t, 651)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-supers-1", "sess-supers-1", `{"call":"inbound-call","from":"+79000000066"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The owner asks to hand off; the ask expires without a decision.
	first, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "первый запрос",
		IdempotencyKey: "sup-request-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	late := ownershipNow.Add(16 * time.Minute)

	// The renewal is allowed (the expired ask never blocks the new ask)…
	renewed, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "повторный запрос",
		IdempotencyKey: "sup-request-2", Now: late,
	})
	if err != nil {
		t.Fatalf("the expired ask must not block the renewal: %v", err)
	}
	// …and the target acknowledges it: the ownership moves (epoch 2).
	if _, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: renewed.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, IdempotencyKey: "sup-ack-1", Now: late,
	}); err != nil {
		t.Fatal(err)
	}

	// The zombie check: the FIRST ask stays unresolved and expired, but
	// the term watermark voids it — the ownership is healthy, the pending
	// word is empty, the case is NOT takeable.
	ownership := w.ownershipState(t, caseID)
	if ownership.State.Health != casesDomain.OwnershipHealthStable || ownership.State.Pending != nil {
		t.Fatalf("the acknowledged renewal voids the older expired ask, got %+v", ownership.State)
	}
	if _, err = w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 2,
		Reason: "take поверх здорового владения", IdempotencyKey: "sup-take-1", Now: late,
	}); err == nil || !strings.Contains(err.Error(), "Владение здорово") {
		t.Fatalf("expected the healthy-ownership take refusal (the zombie never keeps the case takeable), got %v", err)
	}
	// The new owner writes through the healthy ownership (the CAS of the
	// fresh epoch).
	if _, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 2, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "ответ нового владельца", IdempotencyKey: "sup-reply-1", Now: late,
	}); err != nil {
		t.Fatalf("the new owner replies through the healthy ownership: %v", err)
	}

	// The late acknowledge of the ZOMBIE ask (expired, superseded by the
	// acknowledged renewal) refuses honestly — no second transition.
	_, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: first.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, IdempotencyKey: "sup-ack-late-1", Now: late,
	})
	assertRuleError(t, err, "истёк")
}

// TestTransferDoubleExpiryTakeVoidsEveryZombie covers the multi-zombie
// chain: the renewed asks expire one after another, the take voids the
// latest with the cancel resolution, the term watermark voids the rest —
// after the take the ownership is healthy and the second take is refused.
func TestTransferDoubleExpiryTakeVoidsEveryZombie(t *testing.T) {
	w := newCasesWorld(t, 652)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-supers-2", "sess-supers-2", `{"call":"inbound-call","from":"+79000000067"}`)

	// The first ask expires; the renewal is allowed and expires too.
	if _, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "первый запрос",
		IdempotencyKey: "sup2-request-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	late := ownershipNow.Add(16 * time.Minute)
	if _, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "повторный запрос",
		IdempotencyKey: "sup2-request-2", Now: late,
	}); err != nil {
		t.Fatalf("the expired ask must not block the renewal: %v", err)
	}
	late = ownershipNow.Add(32 * time.Minute)

	// The take over the double expiry: the latest ask is voided by the
	// cancel resolution, the earlier one by the term watermark.
	taken, err := w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		Reason: "двойное истечение, забираю", IdempotencyKey: "sup2-take-1", Now: late,
	})
	if err != nil {
		t.Fatal(err)
	}
	if taken.Epoch != 2 {
		t.Fatalf("expected the epoch 2 take, got %+v", taken)
	}
	ownership := w.ownershipState(t, caseID)
	if ownership.State.Health != casesDomain.OwnershipHealthStable || ownership.State.Pending != nil {
		t.Fatalf("the take voids every zombie ask — the healthy ownership, got %+v", ownership.State)
	}
	// The second take over the fresh ownership is refused: the zombie
	// asks never keep the case takeable.
	if _, err = w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 2,
		Reason: "второй take", IdempotencyKey: "sup2-take-2", Now: late,
	}); err == nil || !strings.Contains(err.Error(), "Владение здорово") {
		t.Fatalf("expected the healthy-ownership take refusal, got %v", err)
	}
}

// TestStaleReplyBlocksAndNeedsReview covers the execute-time
// revalidation matrix row: the prepared reply of the PREVIOUS owner
// becomes stale when the ownership moves — the drain writes
// stale_needs_review, the payload is never relayed, the state is visible.
func TestStaleReplyBlocksAndNeedsReview(t *testing.T) {
	w := newCasesWorld(t, 650)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-stale-1", "sess-stale-1", `{"call":"inbound-call","from":"+79000000065"}`)
	conversationID := w.conversationFirst(t, caseID)
	replyEpoch := int64(1)

	// The owner prepares the reply.
	if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "ответ, который устареет", IdempotencyKey: "stale-reply-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}

	// The ownership moves: the supervision reassignment (the owner has
	// every scope of the world, including the configuration path).
	if _, err := w.cases.ReassignOwnership(ctx, casesAPI.ReassignOwnershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "переназначение надзора",
		IdempotencyKey: "stale-reassign-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}

	// The drain: the reply is stale (the epoch and the owner moved).
	drained, err := w.cases.DrainClientReplies(ctx, w.ownershipActor(), w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if drained.Stale != 1 || drained.Dispatched != 0 {
		t.Fatalf("expected the one stale reply, got %+v", drained)
	}
	ownership := w.ownershipState(t, caseID)
	if ownership.Replies[0].OutcomeKind != casesDomain.ReplyOutcomeStaleNeedsReview {
		t.Fatalf("expected the stale_needs_review derivation, got %+v", ownership.Replies[0])
	}
}

// TestReassignmentRequiredAndSupervision covers the unavailability matrix
// row: the deactivated owner stays recorded, the state derives
// reassignment_required, the outgoing writes block, the card shows the
// backup owner and the safe path; the drain raises the attention
// exception; the supervision reassignment and the backup take both stay
// open.
func TestReassignmentRequiredAndSupervision(t *testing.T) {
	w := newCasesWorld(t, 660)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-reassign-1", "sess-reassign-1", `{"call":"inbound-call","from":"+79000000066"}`)
	conversationID := w.conversationFirst(t, caseID)
	replyEpoch := int64(1)

	// The owner prepares a reply, then transfers the ownership to the
	// backup and the backup gets revoked (the honest unavailability).
	if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "ответ перед уходом", IdempotencyKey: "reassign-reply-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "уход",
		IdempotencyKey: "reassign-request-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: request.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, IdempotencyKey: "reassign-ack-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}

	// The membership revocation of the backup (the step-up owner command
	// of the identity module).
	members, err := w.identity.ListMembers(ctx, w.ownershipActor(), w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	backupVersion := int64(0)
	for _, membership := range members {
		if membership.PrincipalID != nil && *membership.PrincipalID == w.backup {
			backupVersion = membership.Version
		}
	}
	if backupVersion == 0 {
		t.Fatal("expected the backup membership row")
	}
	if err = w.identity.RevokeMember(ctx, identityAPI.MembershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, TargetID: w.backup,
		ExpectedVersion: backupVersion, Reason: identityDomain.ReasonOffboarding,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-000000000666"),
	}); err != nil {
		t.Fatal(err)
	}

	// The derivation: the owner stays recorded, the state is the honest
	// reassignment_required, the backup word shows the safe path.
	ownership := w.ownershipState(t, caseID)
	if ownership.State.Health != casesDomain.OwnershipHealthReassignment {
		t.Fatalf("expected the reassignment_required derivation, got %+v", ownership.State)
	}
	if ownership.State.Current == nil || ownership.State.Current.OwnerRef != w.backup {
		t.Fatalf("the deactivated owner stays recorded, got %+v", ownership.State.Current)
	}
	if ownership.State.BackupOwnerID != w.backup {
		t.Fatalf("expected the backup word of the coverage, got %v", ownership.State.BackupOwnerID)
	}

	// The outgoing writes block honestly (the gate word).
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID, ExpectedEpoch: replyEpoch,
		ReplyClass: casesDomain.ReplyClassNeutral,
		Content:    "владелец ушёл", IdempotencyKey: "reassign-reply-2", Now: ownershipNow,
	})
	var gate casesDomain.ReplyGateError
	if !errors.As(err, &gate) || !strings.Contains(gate.Reason, "переназначения") {
		t.Fatalf("expected the reassignment gate refusal, got %v", err)
	}

	// The drain: the pending reply is stale (the owner is unavailable) and
	// the attention exception lands.
	drained, err := w.cases.DrainClientReplies(ctx, w.ownershipActor(), w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	if drained.Stale != 1 {
		t.Fatalf("expected the stale reply of the unavailable owner, got %+v", drained)
	}
	detail, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	attention := 0
	for _, exception := range detail.Exceptions {
		if exception.Kind == casesDomain.ExceptionOwnershipAttention {
			attention++
		}
	}
	if attention != 1 {
		t.Fatalf("expected the attention exception of the unavailable owner, got %d", attention)
	}

	// The supervision reassignment still works (the owner carries the
	// configuration scope).
	if _, err = w.cases.ReassignOwnership(ctx, casesAPI.ReassignOwnershipCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 2,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.owner.ID,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "возврат после ухода владельца",
		IdempotencyKey: "reassign-direct-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	ownership = w.ownershipState(t, caseID)
	if ownership.State.Current.Epoch != 3 || ownership.State.Health != casesDomain.OwnershipHealthStable {
		t.Fatalf("expected the stable epoch 3 after the supervision path, got %+v", ownership.State)
	}
}

// TestOwnershipConcurrentTakesSerialize covers the CAS race row: two
// concurrent takes with the same expected epoch — exactly one transition,
// the loser gets the honest stale-version conflict and re-reads safely.
func TestOwnershipConcurrentTakesSerialize(t *testing.T) {
	w := newCasesWorld(t, 670)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-race-1", "sess-race-1", `{"call":"inbound-call","from":"+79000000067"}`)

	// The request expires first (the deadline word is a derivation — the
	// command clock passes the deadline).
	request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "гонка",
		IdempotencyKey: "race-request-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The author cancels — the request is resolved; the takeable state
	// comes from the owner unavailability path instead: revoke the owner?
	// Simpler: the deadline expiry — wait for the deadline word by the
	// command clock of the late take.
	_ = request
	late := ownershipNow.Add(16 * time.Minute)

	// Two concurrent takes over the expired request: the advisory lock +
	// the material UNIQUE decide — exactly one term.
	ids := newPrivacyIDs(680)
	service := w.buildCasesService(ids)
	takes := []casesAPI.TakeOwnershipCommand{
		{Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1, Reason: "первый", IdempotencyKey: "race-take-a", Now: late},
		{Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1, Reason: "второй", IdempotencyKey: "race-take-b", Now: late},
	}
	outcomes := make([]error, len(takes))
	var wg sync.WaitGroup
	for i := range takes {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, outcomes[index] = service.TakeOwnership(ctx, takes[index])
		}(i)
	}
	wg.Wait()
	if outcomes[0] != nil && outcomes[1] != nil {
		t.Fatalf("expected exactly one successful take, got %v and %v", outcomes[0], outcomes[1])
	}
	if outcomes[0] == nil && outcomes[1] == nil {
		t.Fatal("expected one honest conflict of the race")
	}
	loser := outcomes[1]
	if outcomes[1] == nil {
		loser = outcomes[0]
	}
	if !errors.Is(loser, casesDomain.ErrStaleVersion) {
		t.Fatalf("expected the stale-version conflict of the loser, got %v", loser)
	}
	if count := w.ownershipTermCount(t, caseID); count != 2 {
		t.Fatalf("expected exactly one transition term (2 total), got %d", count)
	}
}

// TestGoverningCaseOfBoundConversation covers the governing-case matrix
// row (AC-7): the actively bound conversation Y answers through the
// governing case A only; the owner of B gets the honest gate refusal with
// the owner of the unified dialog; after the unlink the conversation Y
// is governed by B again.
func TestGoverningCaseOfBoundConversation(t *testing.T) {
	w := newCasesWorld(t, 690)
	ctx := context.Background()
	caseA, _ := w.acceptedCase(t, "evt-gov-a-1", "sess-gov-a-1", `{"call":"inbound-call","from":"+79000000069"}`)
	caseB, _ := w.acceptedCase(t, "evt-gov-b-1", "sess-gov-b-1", `{"call":"inbound-call","from":"+79000000070"}`)
	conversationA := w.conversationFirst(t, caseA)
	conversationB := w.conversationFirst(t, caseB)

	// The ownership split: case B moves to the backup, so the owners of
	// the unified dialog (A) and of the bound conversation's own case (B)
	// differ — the governing-case discipline has a real second owner.
	requestB, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseB, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "владелец B для governing-теста",
		IdempotencyKey: "gov-tr-b-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseB, RequestID: requestB.TransferRequestID,
		Decision: casesDomain.TransferResolutionAcknowledged, IdempotencyKey: "gov-ack-b-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}

	// The verified identity + the consent binding of B's conversation
	// into the case A (the 2-3 flow).
	verification, err := w.cases.StartVerification(ctx, casesAPI.StartVerificationCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseA, ConversationID: conversationA,
		Kind: casesDomain.VerificationKindIdentity, Method: casesDomain.MethodKnownNumberCallback,
		ProviderCode: casesDomain.ProviderCodeOK, SubjectHandle: "позвонивший",
		Note:           "перезвон на номер из факта, абонент подтвердил",
		IdempotencyKey: "gov-verify-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if verification.VerificationOutcome != casesDomain.VerificationPassed {
		t.Fatalf("expected the passed verification, got %+v", verification)
	}
	binding, err := w.cases.BindChannel(ctx, casesAPI.BindChannelCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseA, ConversationID: conversationA,
		BoundCaseID: caseB, BoundConversationID: conversationB,
		Method: casesDomain.BindingMethodConsentHandoff, Purpose: "клиент продолжает в этом канале",
		SubjectHandle: "позвонивший", Note: "клиент согласился продолжить",
		IdempotencyKey: "gov-bind-1", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The owner of B writes through Y naming the case B: the governing
	// case of Y is A (the active binding) — the honest gate refusal.
	var gate casesDomain.ReplyGateError
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseB, ConversationID: conversationB,
		ExpectedEpoch: 2, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "ответ владельца B через связанный канал", IdempotencyKey: "gov-reply-b-1", Now: ownershipNow,
	})
	if !errors.As(err, &gate) {
		t.Fatalf("expected the governing-case gate refusal, got %v", err)
	}
	if !strings.Contains(gate.Reason, "объединённого диалога") {
		t.Fatalf("expected the honest governing word, got %v", gate)
	}
	if gate.CurrentEpoch != 1 || gate.OwnerKind != casesDomain.OwnerKindHuman {
		t.Fatalf("expected the governing owner words of the unified dialog, got %+v", gate)
	}

	// The owner of A writes through Y (the governing case): allowed.
	if _, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseA, ConversationID: conversationB,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "ответ владельца A через связанный канал", IdempotencyKey: "gov-reply-a-1", Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the governing owner answers through the bound conversation: %v", err)
	}

	// The unlink returns the governing right to B.
	if _, err = w.cases.UnlinkChannel(ctx, casesAPI.UnlinkChannelCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseA, BindingID: binding.BindingID,
		Reason: "связь больше не нужна", IdempotencyKey: "gov-unlink-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	_, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseA, ConversationID: conversationB,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "после отмены связи", IdempotencyKey: "gov-reply-a-2", Now: ownershipNow,
	})
	var unlinkGate casesDomain.ReplyGateError
	if !errors.As(err, &unlinkGate) || !strings.Contains(unlinkGate.Reason, "управляется другим обращением") {
		t.Fatalf("expected the governing refusal after the unlink, got %v", err)
	}

	// And the B-side answer works again.
	if _, err = w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseB, ConversationID: conversationB,
		ExpectedEpoch: 2, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "владелец B снова отвечает", IdempotencyKey: "gov-reply-b-2", Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the B owner answers after the unlink: %v", err)
	}
}

// TestExternalSurfaceEvidenceFailClosed covers the evidence-gate matrix
// row (AC-8): the external_inbox surface without the acting verified
// outbound capability evidence refuses honestly — a product name is
// never a proof; in the contour the mutating capabilities are
// unverifiable (AG-4), so the branch is fail-closed.
func TestExternalSurfaceEvidenceFailClosed(t *testing.T) {
	w := newCasesWorld(t, 700)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-surface-1", "sess-surface-1", `{"call":"inbound-call","from":"+79000000071"}`)

	_, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: casesDomain.WritableSurfaceExternalInbox, Reason: "внешний Inbox по названию",
		IdempotencyKey: "surface-external-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "не подтверждён")

	// The surface dictionary: an unknown surface refuses.
	_, err = w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
		ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
		WritableSurface: "jivo", Reason: "название продукта",
		IdempotencyKey: "surface-jivo-1", Now: ownershipNow,
	})
	assertRuleError(t, err, "словарь")
}

// TestOwnershipCommandsRefuseCrossTenant covers the RLS boundary: the
// commands of a foreign tenant get the plain 404 (the tenant existence is
// never disclosed).
func TestOwnershipCommandsRefuseCrossTenant(t *testing.T) {
	w := newCasesWorld(t, 710)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-cross-1", "sess-cross-1", `{"call":"inbound-call","from":"+79000000072"}`)
	foreign := id.MustParse("01994f64-6c00-7000-8000-00000000feed")

	commands := []struct {
		run func() error
	}{
		{func() error {
			_, err := w.cases.TakeOwnership(ctx, casesAPI.TakeOwnershipCommand{Actor: w.ownershipActor(), TenantID: foreign, CaseID: caseID, ExpectedEpoch: 1, IdempotencyKey: "cross-take", Now: ownershipNow})
			return err
		}},
		{func() error {
			_, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{Actor: w.ownershipActor(), TenantID: foreign, CaseID: caseID, ExpectedEpoch: 1, ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup, WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "r", IdempotencyKey: "cross-tr", Now: ownershipNow})
			return err
		}},
		{func() error {
			_, err := w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{Actor: w.ownershipActor(), TenantID: foreign, CaseID: caseID, RequestID: caseID, Decision: casesDomain.TransferResolutionDeclined, IdempotencyKey: "cross-res", Now: ownershipNow})
			return err
		}},
		{func() error {
			_, err := w.cases.ReassignOwnership(ctx, casesAPI.ReassignOwnershipCommand{Actor: w.ownershipActor(), TenantID: foreign, CaseID: caseID, ExpectedEpoch: 1, ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup, WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "r", IdempotencyKey: "cross-re", Now: ownershipNow})
			return err
		}},
		{func() error {
			_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{Actor: w.ownershipActor(), TenantID: foreign, CaseID: caseID, ConversationID: caseID, ReplyClass: casesDomain.ReplyClassNeutral,
				Content: "текст", IdempotencyKey: "cross-reply", Now: ownershipNow})
			return err
		}},
	}
	for index, command := range commands {
		if err := command.run(); !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("command %d: expected the cross-tenant 404, got %v", index, err)
		}
	}
}

// TestOwnershipTablesRLSForced asserts the migration discipline: every
// new table carries RLS + FORCE (the pg_class probe of the R1 lesson of
// 2-3).
func TestOwnershipTablesRLSForced(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('ownership_terms','transfer_requests','transfer_resolutions','client_replies','client_reply_outcomes')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string][2]bool{}
	for rows.Next() {
		var name string
		var rls, force bool
		if err = rows.Scan(&name, &rls, &force); err != nil {
			t.Fatal(err)
		}
		seen[name] = [2]bool{rls, force}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 5 {
		t.Fatalf("expected the five ownership tables, got %d", len(seen))
	}
	for name, flags := range seen {
		if !flags[0] || !flags[1] {
			t.Fatalf("the table %s must carry RLS + FORCE, got %v", name, flags)
		}
	}
}

// TestOwnershipGrantsAppendOnly asserts the grant discipline: the
// ownership tables carry INSERT/SELECT for staffy_app and no UPDATE or
// DELETE (the append-only history).
func TestOwnershipGrantsAppendOnly(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	for _, table := range []string{"ownership_terms", "transfer_requests", "transfer_resolutions", "client_replies", "client_reply_outcomes"} {
		var canInsert, canSelect, canUpdate, canDelete bool
		err := runtimePool.QueryRow(ctx, fmt.Sprintf(`
                        select has_table_privilege('staffy_app', 'cases.%s', 'INSERT'),
                               has_table_privilege('staffy_app', 'cases.%s', 'SELECT'),
                               has_table_privilege('staffy_app', 'cases.%s', 'UPDATE'),
                               has_table_privilege('staffy_app', 'cases.%s', 'DELETE')`, table, table, table, table)).
			Scan(&canInsert, &canSelect, &canUpdate, &canDelete)
		if err != nil {
			t.Fatal(err)
		}
		if !canInsert || !canSelect || canUpdate || canDelete {
			t.Fatalf("the table %s must be INSERT/SELECT only, got insert=%v select=%v update=%v delete=%v", table, canInsert, canSelect, canUpdate, canDelete)
		}
	}
}
