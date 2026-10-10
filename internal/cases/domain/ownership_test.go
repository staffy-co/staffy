package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/artemiosu/staffy/internal/platform/id"
)

// ownershipBase builds the deterministic material of the ownership tests:
// a case with the ingest term (epoch 1, the human owner A) over the fixed
// clock word.
func ownershipBase() ([]OwnershipTerm, []TransferRequest, []TransferResolution, time.Time) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	terms := []OwnershipTerm{{
		TenantID: id.MustParse("00000000-0000-7000-8000-0000000000a1"),
		CaseID:   id.MustParse("00000000-0000-7000-8000-0000000000b1"),
		TermID:   id.MustParse("00000000-0000-7000-8000-0000000000c1"),
		Epoch:    1, OwnerKind: OwnerKindHuman,
		OwnerRef:        id.MustParse("00000000-0000-7000-8000-0000000000d1"),
		WritableSurface: WritableSurfaceInbox, Origin: OwnershipOriginIngestInitial,
		CreatedAt: now.Add(-time.Hour),
	}}
	return terms, nil, nil, now
}

func TestOwnershipCurrentTermIsMaxEpoch(t *testing.T) {
	terms, _, _, _ := ownershipBase()
	if current := CurrentOwnership(terms); current == nil || current.Epoch != 1 {
		t.Fatalf("expected the epoch 1 term, got %+v", current)
	}
	terms = append(terms, OwnershipTerm{
		TermID: id.MustParse("00000000-0000-7000-8000-0000000000c2"),
		CaseID: terms[0].CaseID, Epoch: 3, OwnerKind: OwnerKindHuman,
		OwnerRef:        id.MustParse("00000000-0000-7000-8000-0000000000d2"),
		WritableSurface: WritableSurfaceInbox, Origin: OwnershipOriginTake,
		PreviousOwnerRef: terms[0].OwnerRef, PreviousOwnerKind: OwnerKindHuman,
	}, OwnershipTerm{
		TermID: id.MustParse("00000000-0000-7000-8000-0000000000c3"),
		CaseID: terms[0].CaseID, Epoch: 2, OwnerKind: OwnerKindHuman,
		OwnerRef:        id.MustParse("00000000-0000-7000-8000-0000000000d3"),
		WritableSurface: WritableSurfaceInbox, Origin: OwnershipOriginReassign,
		PreviousOwnerRef: terms[0].OwnerRef, PreviousOwnerKind: OwnerKindHuman,
	})
	if current := CurrentOwnership(terms); current.Epoch != 3 {
		t.Fatalf("expected the maximal epoch 3, got %d", current.Epoch)
	}
	if current := CurrentOwnership(nil); current != nil {
		t.Fatalf("the empty history has no current term, got %+v", current)
	}
}

func TestOwnershipHealthDerivations(t *testing.T) {
	terms, _, _, now := ownershipBase()
	deadline := now.Add(15 * time.Minute)
	request := TransferRequest{
		RequestID: id.MustParse("00000000-0000-7000-8000-0000000000e1"),
		CaseID:    terms[0].CaseID, RequestNo: 1,
		FromOwnerKind: OwnerKindHuman, FromOwnerRef: terms[0].OwnerRef,
		ToOwnerKind: OwnerKindHuman, ToOwnerRef: id.MustParse("00000000-0000-7000-8000-0000000000d2"),
		WritableSurface: WritableSurfaceInbox, Reason: "отпуск",
		DeadlineAt: deadline, CreatedAt: now.Add(-time.Minute),
	}

	// stable: the active owner, no request.
	state := DeriveOwnershipState(terms, nil, nil, true, now)
	if state.Health != OwnershipHealthStable || state.Pending != nil {
		t.Fatalf("expected the stable health, got %+v", state)
	}
	if state.Takeable() || state.ReplyBlocked() {
		t.Fatal("the stable ownership is neither takeable nor reply-blocked")
	}

	// transfer_pending: the request is inside its deadline; the current
	// owner keeps the reply right (AC-5).
	state = DeriveOwnershipState(terms, []TransferRequest{request}, nil, true, now)
	if state.Health != OwnershipHealthTransferPending || state.Pending == nil {
		t.Fatalf("expected the pending transfer, got %+v", state)
	}
	if state.Takeable() {
		t.Fatal("a pending request inside the deadline is not takeable")
	}
	if state.ReplyBlocked() {
		t.Fatal("a pending request never blocks the current owner's replies")
	}

	// transfer_timed_out: the deadline passed without a decision; the
	// request is voided by the derivation and the take opens.
	state = DeriveOwnershipState(terms, []TransferRequest{request}, nil, true, deadline.Add(time.Minute))
	if state.Health != OwnershipHealthTransferTimedOut || !state.PendingTimedOut {
		t.Fatalf("expected the timed-out transfer, got %+v", state)
	}
	if !state.Takeable() {
		t.Fatal("the expired request opens the take")
	}

	// the resolution voids the request even after the deadline: no
	// zombie pending state (the decline keeps the owner).
	resolution := TransferResolution{RequestID: request.RequestID, Kind: TransferResolutionDeclined}
	state = DeriveOwnershipState(terms, []TransferRequest{request}, []TransferResolution{resolution}, true, deadline.Add(time.Minute))
	if state.Health != OwnershipHealthStable || state.Pending != nil {
		t.Fatalf("the resolved request must leave the stable state, got %+v", state)
	}

	// reassignment_required: the owner is unavailable — the replies
	// block, the take and the reassign stay open.
	state = DeriveOwnershipState(terms, nil, nil, false, now)
	if state.Health != OwnershipHealthReassignment {
		t.Fatalf("expected the reassignment state, got %+v", state)
	}
	if !state.Takeable() || !state.ReplyBlocked() {
		t.Fatal("the unavailable owner opens the take and blocks the replies")
	}

	// the staffy owner is never "unavailable" by the member word.
	staffyTerms := append([]OwnershipTerm(nil), terms[0])
	staffyTerms[0].OwnerKind = OwnerKindStaffy
	state = DeriveOwnershipState(staffyTerms, nil, nil, false, now)
	if state.Health != OwnershipHealthStable {
		t.Fatalf("the staffy ownership does not derive the reassignment, got %+v", state)
	}
}

func TestExternalSurfaceEvidenceGate(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	word := []SurfaceCapability{{Direction: "outbound", Operation: "command", State: "verified"}}
	if !ExternalSurfaceEvidenced(word, now) {
		t.Fatal("the verified outbound command capability evidences the surface")
	}
	word[0].Operation = "event"
	if !ExternalSurfaceEvidenced(word, now) {
		t.Fatal("the verified outbound event capability evidences the surface")
	}
	word[0].HasExpiry, word[0].ExpiresAt = true, now.Add(time.Hour)
	if !ExternalSurfaceEvidenced(word, now) {
		t.Fatal("the unexpired verified capability evidences the surface")
	}
	// the negative branches: the direction, the operation, the state,
	// the expiry and the empty word (fail-closed).
	negatives := [][]SurfaceCapability{
		{{Direction: "inbound", Operation: "command", State: "verified"}},
		{{Direction: "outbound", Operation: "query", State: "verified"}},
		{{Direction: "outbound", Operation: "command", State: "unverified"}},
		{{Direction: "outbound", Operation: "command", State: "stale"}},
		{{Direction: "outbound", Operation: "command", State: "verified", HasExpiry: true, ExpiresAt: now.Add(-time.Minute)}},
		nil,
	}
	for _, negative := range negatives {
		if ExternalSurfaceEvidenced(negative, now) {
			t.Fatalf("the word %+v must not evidence the surface", negative)
		}
	}
}

func TestOwnershipCommandGuards(t *testing.T) {
	if err := GuardRequestTransfer(OwnerKindHuman, WritableSurfaceExternalInbox, "командировка"); err != nil {
		t.Fatalf("the valid material must pass, got %v", err)
	}
	if err := GuardRequestTransfer("queue", WritableSurfaceInbox, "причина"); err == nil {
		t.Fatal("a queue is never an owner kind")
	}
	if err := GuardRequestTransfer(OwnerKindHuman, "jivo", "причина"); err == nil {
		t.Fatal("a product name is never a surface")
	}
	if err := GuardRequestTransfer(OwnerKindHuman, WritableSurfaceInbox, ""); err == nil {
		t.Fatal("the reason is required")
	}
	if err := GuardRequestTransfer(OwnerKindHuman, WritableSurfaceInbox, strings.Repeat("д", MaxOwnershipReasonRunes+1)); err == nil {
		t.Fatal("the reason is bounded")
	}
	if err := GuardResolveTransfer(TransferResolutionAcknowledged, ""); err != nil {
		t.Fatalf("the valid resolution must pass, got %v", err)
	}
	if err := GuardResolveTransfer("timeout", ""); err == nil {
		t.Fatal("the timeout is a derivation, never a stored resolution kind")
	}
	if err := GuardSendClientReply(1, "Добрый день! Ваше обращение принято."); err != nil {
		t.Fatalf("the valid reply must pass, got %v", err)
	}
	if err := GuardSendClientReply(0, "текст"); err == nil {
		t.Fatal("the expected epoch is required")
	}
	if err := GuardSendClientReply(1, "   "); err == nil {
		t.Fatal("the empty content is refused")
	}
	if err := GuardSendClientReply(1, strings.Repeat("а", MaxReplyContentRunes+1)); err == nil {
		t.Fatal("the content is bounded")
	}
}

func TestOwnershipGateRefusalText(t *testing.T) {
	refusal := OwnershipGateRefusal{
		OwnerKind: OwnerKindHuman, OwnerName: "Анна", CurrentEpoch: 3,
		PermittedPath: "Запросите передачу (request_transfer).",
	}
	text := refusal.Text()
	for _, word := range []string{"Анна", "3", "request_transfer"} {
		if !strings.Contains(text, word) {
			t.Fatalf("the refusal must carry %q, got %q", word, text)
		}
	}
	refusal.OwnerKind = OwnerKindStaffy
	if !strings.Contains(refusal.Text(), "цифровой сотрудник") {
		t.Fatal("the staffy kind renders honestly")
	}
}

func TestOwnershipOutboxPayloadsCarryNoContent(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	tenant := id.MustParse("00000000-0000-7000-8000-0000000000a1")
	caseID := id.MustParse("00000000-0000-7000-8000-0000000000b1")
	replyID := id.MustParse("00000000-0000-7000-8000-0000000000f1")
	secret := "СЕКРЕТНЫЙ-ТЕКСТ-ОТВЕТА"

	prepared, err := ClientReplyPreparedPayloadJSON(ClientReplyPreparedPayload{
		TenantID: tenant, CaseID: caseID, ReplyID: replyID,
		OwnerKind: OwnerKindHuman, OwnerRef: id.MustParse("00000000-0000-7000-8000-0000000000d1"),
		Epoch: 2, WritableSurface: WritableSurfaceInbox,
		ConnectionID: id.MustParse("00000000-0000-7000-8000-000000000011"), Provider: "p", Channel: "phone",
		OccurredAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(prepared), secret) || strings.Contains(string(prepared), "content") {
		t.Fatalf("the prepared envelope must never carry the content fields, got %s", prepared)
	}
	var decoded map[string]any
	if err := json.Unmarshal(prepared, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"tenant_id", "case_id", "conversation_id", "reply_id", "epoch", "writable_surface", "channel"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("the prepared envelope must carry %q", field)
		}
	}

	dispatched, err := ClientReplyDispatchedPayloadJSON(ClientReplyDispatchedPayload{
		TenantID: tenant, CaseID: caseID, ReplyID: replyID,
		Kind: ReplyOutcomeDispatched, OccurredAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(dispatched), "content") {
		t.Fatalf("the dispatched envelope must never carry the content, got %s", dispatched)
	}

	changed, err := OwnershipChangedPayloadJSON(OwnershipChangedPayload{
		TenantID: tenant, CaseID: caseID, TermID: id.MustParse("00000000-0000-7000-8000-0000000000c2"),
		Epoch: 2, OwnerKind: OwnerKindHuman, OwnerRef: id.MustParse("00000000-0000-7000-8000-0000000000d1"),
		WritableSurface: WritableSurfaceInbox, Origin: OwnershipOriginTake,
		ActorID: id.MustParse("00000000-0000-7000-8000-0000000000d1"), OccurredAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changed), `"epoch":2`) {
		t.Fatalf("the ownership event must carry the epoch, got %s", changed)
	}
	if strings.Contains(string(changed), "reason") {
		t.Fatalf("the ownership event never carries the reason text, got %s", changed)
	}
}

func TestPendingTransferPicksLatestOpenRequest(t *testing.T) {
	_, _, _, now := ownershipBase()
	first := TransferRequest{RequestID: id.MustParse("00000000-0000-7000-8000-0000000000e1"), CreatedAt: now.Add(-2 * time.Hour)}
	second := TransferRequest{RequestID: id.MustParse("00000000-0000-7000-8000-0000000000e2"), CreatedAt: now.Add(-time.Hour)}
	if pending := PendingTransfer([]TransferRequest{first, second}, nil, now); pending == nil || pending.RequestID != second.RequestID {
		t.Fatalf("the latest open request wins, got %+v", pending)
	}
	resolved := TransferResolution{RequestID: second.RequestID, Kind: TransferResolutionCancelled}
	if pending := PendingTransfer([]TransferRequest{first, second}, []TransferResolution{resolved}, now); pending == nil || pending.RequestID != first.RequestID {
		t.Fatalf("the resolved request leaves the older open one, got %+v", pending)
	}
}
