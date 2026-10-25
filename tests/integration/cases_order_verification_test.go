//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	actionAPI "github.com/artemiosu/staffy/internal/action/api"
	actionApp "github.com/artemiosu/staffy/internal/action/app"
	actionDomain "github.com/artemiosu/staffy/internal/action/domain"
	actionStore "github.com/artemiosu/staffy/internal/action/store"
	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.7 (step-04): the manager order verification eval matrix — the
// AC-15 rows LIVE over the order world of 4.6 (the committed receipt on
// the entry). Every row owns its own case: the per-row 4-2..4-5 chain,
// the builder proposal, the confirmation, the authorization and the
// committed dispatch — then the verification lifecycle walks the row's
// honest paths.
// ---------------------------------------------------------------------------

// verificationWorld wires the order world with the verification contour
// (the same service of the 4-6 matrix — the WithOrderVerifications wiring
// of buildOrderService).
func verificationWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, identityDomain.Principal, identityDomain.Principal) {
	t.Helper()
	w, svc, adapter, _, _, operator := orderWorld(t, seed)
	// The backup manager of the pilot (the invited expert_qa member with
	// the review scope — the profile world's backup).
	backup := identityDomain.Principal{ID: w.backup}
	return w, svc, adapter, operator, backup
}

// verificationCommittedOrder walks the full chain of one row's case to
// the COMMITTED order receipt: the row case, the builder proposal, the
// confirmation, the authorization and the synthetic dispatch with the
// external ABCP Order ID (the assign anchor material).
func verificationCommittedOrder(t *testing.T, w *casesWorld, svc *actionApp.Service, adapter *countingMutationAdapter,
	operator identityDomain.Principal, tag string, key string) (caseID, workItemID id.UUID, operation actionDomain.Operation, digest string) {
	t.Helper()
	ctx := context.Background()
	caseID, workItemID = orderRowCase(t, w, tag)
	built, _, operation := orderConfirmAuthorize(t, w, svc, operator, caseID, workItemID, key, tag)
	// The pinned digest of the operation's preview (the expected digest of
	// the approve command — read from the LIVE preview card).
	thread, err := svc.ReadPreviewView(ctx, operator, w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, threadView := range thread.Threads {
		for _, card := range threadView.Cards {
			if card.PreviewID == built.PreviewID && card.VersionNo == 1 {
				digest = card.TypedInput["order_digest"]
			}
		}
	}
	if len(digest) != 64 {
		t.Fatalf("the pinned digest of the operation, got %q", digest)
	}
	// The synthetic dispatch: committed with the external triple.
	adapter.inner.Script(actionDomain.MutationOutcomeCommitted, "corr-verify-"+tag, nil)
	adapter.inner.ScriptEvidence("ABCP-ORDER-"+tag, "3")
	adapter.reset()
	report, err := svc.DispatchQueuedOperations(ctx, actionApp.DispatchDrainCommand{
		Actor: operator, TenantID: w.tenant, Now: orderNow.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Dispatched != 1 {
		t.Fatalf("the committed order dispatch of the row %q, got %+v", tag, report)
	}
	return caseID, workItemID, operation, digest
}

// verificationAssign runs the passing assignment of one row (the operator
// assigns the manager; the backup rides the round).
func verificationAssign(t *testing.T, svc *actionApp.Service, operator, manager, backup identityDomain.Principal,
	w *casesWorld, caseID, workItemID id.UUID, operation actionDomain.Operation, deadline string, now time.Time) actionAPI.OrderVerificationOutcome {
	t.Helper()
	outcome, err := svc.AssignOrderVerification(context.Background(), actionAPI.AssignOrderVerificationCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		OperationID: operation.OperationID, Assignee: manager.ID, BackupOwner: backup.ID,
		Deadline: deadline, Now: now,
	})
	if err != nil {
		t.Fatalf("the assignment of the verification task refused: %v", err)
	}
	return outcome
}

// verificationEvents reads the events of one task over the admin truth
// (the append-only rows — the SQL backstop of every lifecycle assertion).
func verificationEvents(t *testing.T, tenant, caseID id.UUID, verificationNo int64) []struct {
	EventNo  int64
	Kind     string
	Digest   string
	Anchors  string
	Reason   string
	Position string
} {
	t.Helper()
	rows, err := adminPool.Query(context.Background(),
		`select event_no, kind, coalesce(order_digest,''), coalesce(evidence_anchors::text,''),
                        coalesce(reason,''), coalesce(positions::text,'')
                 from action.order_verification_events
                 where tenant_id=$1 and case_id=$2 and verification_no=$3 order by event_no`,
		tenant, caseID, verificationNo)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	events := []struct {
		EventNo  int64
		Kind     string
		Digest   string
		Anchors  string
		Reason   string
		Position string
	}{}
	for rows.Next() {
		var event struct {
			EventNo  int64
			Kind     string
			Digest   string
			Anchors  string
			Reason   string
			Position string
		}
		if err := rows.Scan(&event.EventNo, &event.Kind, &event.Digest, &event.Anchors, &event.Reason, &event.Position); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	return events
}

// TestOrderVerificationMatrixRowsBySpec walks the Story 4.7 eval matrix
// LIVE: the full chain row (assign→acknowledge→approve with the Human
// Verification Receipt pins BY NAME), the undelivered notification word
// separation (AC-2), the decision without the acceptance (AC-7), the
// expired deadline (the overdue derivation, NO automatic approval —
// structurally, the backup word, the reassignment fallback, AC-8), the
// concurrent decision of two managers (exactly one wins — CAS, AC-7), the
// stale order (the live digest divergence refuses the decision BY NAME,
// AC-9), the wrong position of the correction decision (AC-12), the
// supplier-send structural probe (OQ-3 — zero), the RLS blindness of the
// foreign tenant and the replay ladder (the new task after the decision,
// AC-14).
func TestOrderVerificationMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	// The world seed 759: the order family range (757 = the 4-6 matrix)
	// — the membership identities of the profile world derive from the
	// seed, so a UNIQUE seed keeps the shared-DB worlds collision-free
	// (the 767 of the first draft collided with the policy world of the
	// same seed — the membership version conflict of the later file).
	w, svc, adapter, operator, backup := verificationWorld(t, 759)

	// Row 1 — the FULL live chain: the committed order, the assignment,
	// the acceptance and the approved decision — the Human Verification
	// Receipt with the material pins BY NAME (the digest of the exact
	// version, the receipt identity, the external ABCP Order ID, the
	// evidence anchors).
	t.Run("row-1 full chain: assign→acknowledge→approve receipt", func(t *testing.T) {
		caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, "v1", fmtOrderTarget(901))
		assign := verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		if assign.Task.VerificationNo != 1 || assign.Task.OperationID != operation.OperationID {
			t.Fatalf("the task 1 anchors the operation, got %+v", assign.Task)
		}
		if assign.Task.ReceiptVersionNo != 1 || assign.Task.ExternalOrderID != "ABCP-ORDER-v1" {
			t.Fatalf("the receipt pins of the task, got %+v", assign.Task)
		}
		if assign.View.State != actionDomain.OrderVerificationStateAssigned || assign.StateText != actionDomain.OrderVerificationDeliveryWord {
			t.Fatalf("the assigned state with the delivery word, got %q / %q", assign.View.State, assign.StateText)
		}
		ack, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
		})
		if err != nil || ack.View.State != actionDomain.OrderVerificationStateAcknowledged {
			t.Fatalf("the acceptance, got %+v err=%v", ack, err)
		}
		approved, err := svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: orderNow.Add(22 * time.Minute),
		})
		if err != nil {
			t.Fatalf("the approved decision refused: %v", err)
		}
		if approved.View.State != actionDomain.OrderVerificationStateApproved {
			t.Fatalf("the approved state, got %q", approved.View.State)
		}
		if approved.StateText != actionDomain.OrderVerificationManualContourWord {
			t.Fatalf("the MANDATORY manual-contour word of the outcome, got %q", approved.StateText)
		}
		// The SQL truth of the Human Verification Receipt: three events,
		// the approved row carries the digest, the anchors and NO reason.
		events := verificationEvents(t, w.tenant, caseID, 1)
		if len(events) != 3 || events[0].Kind != "assigned" || events[1].Kind != "acknowledged" || events[2].Kind != "approved" {
			t.Fatalf("the lifecycle events, got %+v", events)
		}
		if events[2].Digest != digest {
			t.Fatalf("the approved digest pin, got %q want %q", events[2].Digest, digest)
		}
		if !strings.Contains(events[2].Anchors, "position:WF2B196K666AA") ||
			!strings.Contains(events[2].Anchors, "receipt:1") {
			t.Fatalf("the evidence anchors of the receipt, got %q", events[2].Anchors)
		}
		if events[2].Reason != "" || events[2].Position != "" {
			t.Fatalf("the approved event carries NO block material, got %q / %q", events[2].Reason, events[2].Position)
		}
		// The read-time applicability of the fresh decision: the live
		// digest still equals the pin — applicable.
		views, err := svc.ReadOrderVerificationCaseView(ctx, operator, w.tenant, caseID, orderNow.Add(23*time.Minute))
		if err != nil || len(views) != 1 {
			t.Fatalf("the case view of the task, got %v err=%v", views, err)
		}
		if views[0].State != actionDomain.OrderVerificationStateApproved || views[0].Decision == nil || views[0].Decision.OrderDigest != digest {
			t.Fatalf("the approved view with the decision pin, got %+v", views[0])
		}
	})

	// Row 2 — the UNDELIVERED notification (AC-2): the assignment is NOT
	// the delivery and NOT the acceptance — the derived words of the
	// assigned round never claim «уведомлён»; the delivery boundary word
	// separates the three truths; the word does not exist in the
	// dictionary.
	t.Run("row-2 undelivered notification: назначен ≠ доставлено ≠ принял", func(t *testing.T) {
		caseID, workItemID, operation, _ := verificationCommittedOrder(t, w, svc, adapter, operator, "v2", fmtOrderTarget(902))
		assign := verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		// The derived words of the assigned round: NO delivery claim.
		for _, word := range []string{assign.Word, assign.StateText} {
			if strings.Contains(strings.ToLower(word), "уведомлён") {
				t.Fatalf("the assigned round word claims a delivery: %q", word)
			}
		}
		if assign.StateText != actionDomain.OrderVerificationDeliveryWord {
			t.Fatalf("the delivery boundary word of the assignment, got %q", assign.StateText)
		}
		// The state dictionary carries NO delivery word at all (the
		// honest boundary: the external channel does not exist).
		for _, state := range actionDomain.OrderVerificationStates {
			if state == "delivered" || state == "notified" {
				t.Fatalf("the state dictionary carries a delivery word: %q", state)
			}
		}
		// The acknowledgment word differs from the assignment word (two
		// truths, never one).
		if actionDomain.OrderVerificationAssignedWord == actionDomain.OrderVerificationAcknowledgedWord {
			t.Fatal("the assignment and the acceptance are ONE word")
		}
	})

	// Row 3 — the decision WITHOUT the acceptance (AC-7): the approve and
	// the block of the assigned (not accepted) round refuse with the
	// honest «менеджер ещё не принял задачу проверки» word; the events
	// stay one.
	t.Run("row-3 decision without acknowledgement refused", func(t *testing.T) {
		caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, "v3", fmtOrderTarget(903))
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		_, err := svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: orderNow.Add(21 * time.Minute),
		})
		if err == nil || err.Error() != actionDomain.OrderVerificationNotAcknowledgedWord {
			t.Fatalf("the approve without the acceptance, got %v", err)
		}
		_, err = svc.BlockOrderVerification(ctx, actionAPI.BlockOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Positions: []string{"WF2B196K666AA"}, Reason: actionDomain.VerifyBlockReasonWrongPart,
			Now: orderNow.Add(21 * time.Minute),
		})
		if err == nil || err.Error() != actionDomain.OrderVerificationNotAcknowledgedWord {
			t.Fatalf("the block without the acceptance, got %v", err)
		}
		if events := verificationEvents(t, w.tenant, caseID, 1); len(events) != 1 {
			t.Fatalf("the refused decisions write NO events, got %+v", events)
		}
		// The NOT-ASSIGNEE actor (the backup holds the review scope but is
		// NOT the assignee): the honest actor word.
		_, err = svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: backup, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
		})
		if err == nil || err.Error() != actionDomain.OrderVerificationNotAssigneeWord {
			t.Fatalf("the not-assignee acceptance, got %v", err)
		}
	})

	// Row 4 — the EXPIRED deadline (AC-8): the overdue derivation of the
	// open round after the deadline (the read-time word with the
	// reassignment path and the backup by name); the automatic approval
	// does NOT exist STRUCTURALLY (no event lands); the decision of the
	// expired round refuses; the owner's reassignment opens the new round
	// and the check completes there (the fallback path).
	t.Run("row-4 expired deadline: overdue, no auto-approval, reassign fallback", func(t *testing.T) {
		caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, "v4", fmtOrderTarget(904))
		assigned := orderNow.Add(20 * time.Minute)
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, assigned)
		expired := assigned.Add(4*time.Hour + time.Minute)
		// The overdue derivation of the open round.
		views, err := svc.ReadOrderVerificationCaseView(ctx, operator, w.tenant, caseID, expired)
		if err != nil || len(views) != 1 || views[0].State != actionDomain.OrderVerificationStateOverdue {
			t.Fatalf("the overdue derivation, got %+v err=%v", views, err)
		}
		if actionDomain.OrderVerificationStateText(actionDomain.OrderVerificationStateOverdue) != actionDomain.OrderVerificationOverdueWord {
			t.Fatal("the overdue word carries the honest text")
		}
		// The automatic approval does not exist: no event landed after the
		// deadline (the structural probe — the expiry writes nothing).
		if events := verificationEvents(t, w.tenant, caseID, 1); len(events) != 1 {
			t.Fatalf("the expiry writes NO event, got %+v", events)
		}
		// The decision of the expired round refuses with the honest path.
		_, err = svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: expired,
		})
		if err == nil || !strings.Contains(err.Error(), "переназначение") {
			t.Fatalf("the expired decision refuses with the reassignment path, got %v", err)
		}
		// The owner's reassignment opens the NEW round (the backup by name
		// rides the round) — the fallback, never an automatic takeover.
		reassigned, err := svc.ReassignOrderVerification(ctx, actionAPI.ReassignOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			Assignee: operator.ID, BackupOwner: backup.ID, Deadline: actionDomain.Verify1d, Now: expired,
		})
		if err != nil || reassigned.View.AssignmentEpoch != 2 {
			t.Fatalf("the reassignment opens the new epoch, got %+v err=%v", reassigned, err)
		}
		// The check completes over the new round.
		if _, err = svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 2, Now: expired.Add(time.Minute),
		}); err != nil {
			t.Fatalf("the acceptance of the new round, got %v", err)
		}
		if _, err = svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 2, ExpectedOrderDigest: digest, Now: expired.Add(2 * time.Minute),
		}); err != nil {
			t.Fatalf("the decision of the new round, got %v", err)
		}
		if events := verificationEvents(t, w.tenant, caseID, 1); len(events) != 4 {
			t.Fatalf("the full fallback ladder (assigned, assigned, acknowledged, approved), got %+v", events)
		}
	})

	// Row 5 — the CONCURRENT decision of two managers (AC-7): two
	// approve commands of the same epoch race — EXACTLY ONE wins (the CAS
	// under the advisory lock), the loser answers the honest raced word
	// with the CURRENT state; one approved event lands.
	t.Run("row-5 concurrent decision: exactly one wins CAS", func(t *testing.T) {
		caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, "v5", fmtOrderTarget(905))
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		if _, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		now := orderNow.Add(22 * time.Minute)
		var wg sync.WaitGroup
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
					Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
					ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: now,
				})
				results <- err
			}()
		}
		wg.Wait()
		close(results)
		wins, losses := 0, 0
		for err := range results {
			if err == nil {
				wins++
			} else {
				var raced actionDomain.OrderVerificationRacedError
				if !errors.As(err, &raced) {
					t.Fatalf("the loser of the race answers the raced word, got %v", err)
				}
				losses++
			}
		}
		if wins != 1 || losses != 1 {
			t.Fatalf("exactly one winner and one loser, got %d/%d", wins, losses)
		}
		events := verificationEvents(t, w.tenant, caseID, 1)
		if len(events) != 3 || events[2].Kind != "approved" {
			t.Fatalf("ONE approved event of the race, got %+v", events)
		}
		// The stale-epoch acknowledge AFTER the decision: the honest
		// closed-state refusal (the task is decided — nothing to accept).
		_, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: now,
		})
		if err == nil || !strings.Contains(err.Error(), "принятие") {
			t.Fatalf("the stale acknowledge of the decided task refuses with the state word, got %v", err)
		}
	})

	// Row 6 — the STALE order (AC-9): the re-recorded offer price changes
	// the LIVE digest — the approve command is refused BY NAME with the
	// changed-composition word; the decision event is NEVER written; the
	// live material derivation pins the divergence.
	t.Run("row-6 stale order: live digest refuses the decision BY NAME", func(t *testing.T) {
		caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, "v6", fmtOrderTarget(906))
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		if _, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		// The re-recorded offer: the CURRENT record of the supplier moves
		// the price (the live composition changes).
		offerRecord(t, w, caseID, workItemID, "verify-stale", func(command *casesAPI.RecordSupplierOfferCommand) {
			command.SupplierID = "SUP-2"
			command.SupplierName = "Поставщик два"
			command.Brand = "Mann"
			command.ClientPrice = 4300
			command.PurchasePrice = nil
			command.QuantityAvailable = 6
			command.IdempotencyKey = "verify-stale-offer"
			command.SourceRecordID = "rec-verify-stale"
		})
		live, err := svc.ReadOrderVerificationLiveMaterial(ctx, operator, w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		if liveDigest := actionDomain.OrderDigestOf(live); liveDigest == digest {
			t.Fatal("the re-recorded price must change the live digest")
		}
		_, err = svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: orderNow.Add(22 * time.Minute),
		})
		if !errors.Is(err, actionDomain.ErrOrderVerificationCompositionChanged) {
			t.Fatalf("the stale decision refuses BY NAME, got %v", err)
		}
		if events := verificationEvents(t, w.tenant, caseID, 1); len(events) != 2 {
			t.Fatalf("the refused decision writes NO event, got %+v", events)
		}
	})

	// Row 7 — the WRONG position of the correction decision (AC-12): the
	// foreign position key refuses with the composition word; the VALID
	// position with the reason word lands the blocked event (the
	// positions, the reason) — the original evidence never changes.
	t.Run("row-7 wrong position refused, valid block recorded", func(t *testing.T) {
		caseID, workItemID, operation, _ := verificationCommittedOrder(t, w, svc, adapter, operator, "v7", fmtOrderTarget(907))
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		if _, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		_, err := svc.BlockOrderVerification(ctx, actionAPI.BlockOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Positions: []string{"pos-9"}, Reason: actionDomain.VerifyBlockReasonWrongPart,
			Now: orderNow.Add(22 * time.Minute),
		})
		if err == nil || !strings.Contains(err.Error(), "состав операции") {
			t.Fatalf("the foreign position refuses with the composition word, got %v", err)
		}
		blocked, err := svc.BlockOrderVerification(ctx, actionAPI.BlockOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Positions: []string{"WF2B196K666AA"}, Reason: actionDomain.VerifyBlockReasonPriceChanged,
			Now: orderNow.Add(23 * time.Minute),
		})
		if err != nil {
			t.Fatalf("the valid block refused: %v", err)
		}
		if blocked.View.State != actionDomain.OrderVerificationStateBlocked {
			t.Fatalf("the blocked state, got %q", blocked.View.State)
		}
		if blocked.StateText != actionDomain.OrderVerificationCorrectionWord {
			t.Fatalf("the correction launch word of the outcome, got %q", blocked.StateText)
		}
		events := verificationEvents(t, w.tenant, caseID, 1)
		if len(events) != 3 || events[2].Kind != "blocked" {
			t.Fatalf("the blocked event, got %+v", events)
		}
		if events[2].Reason != actionDomain.VerifyBlockReasonPriceChanged {
			t.Fatalf("the reason word of the blocked event, got %q", events[2].Reason)
		}
		if !strings.Contains(events[2].Position, "WF2B196K666AA") {
			t.Fatalf("the positions of the blocked event, got %q", events[2].Position)
		}
		if events[2].Digest != "" || events[2].Anchors != "" {
			t.Fatalf("the blocked event carries NO approve material, got %q / %q", events[2].Digest, events[2].Anchors)
		}
	})

	// Row 8 — the AUTOMATIC SEND attempt (AC-15/OQ-3): the structural
	// probe — NO supplier-send column, field, route or word exists in the
	// verification contour; the approval NEVER sends anything (the
	// adapter of the world stays at ZERO calls after the decision).
	t.Run("row-8 supplier-send probe: zero", func(t *testing.T) {
		caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, "v8", fmtOrderTarget(908))
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		if _, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		adapter.reset()
		if _, err := svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: orderNow.Add(22 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if adapter.total() != 0 {
			t.Fatalf("the approval sends NOTHING, adapter calls %d", adapter.total())
		}
		// The SQL structural probe: no send/supplier column exists on the
		// verification tables.
		rows, err := adminPool.Query(ctx,
			`select column_name from information_schema.columns
                         where table_schema='action' and table_name in ('order_verifications','order_verification_events')
                         and (column_name ilike '%send%' or column_name ilike '%supplier%')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if rows.Next() {
			t.Fatal("the verification tables carry a supplier-send column")
		}
		// The domain probe: the honest words that mention the send are the
		// NEGATIONS only (the manual contour, the fence honesty).
		for _, word := range []string{
			actionDomain.OrderVerificationManualContourWord, actionDomain.OrderVerificationFenceWord,
		} {
			if !strings.Contains(word, "не отправляет") && !strings.Contains(word, "не существует") {
				t.Fatalf("the send-mentioning word must be the honest negation: %q", word)
			}
		}
	})

	// Row 9 — the RLS BLINDNESS of the foreign tenant: the verification
	// reads of a foreign tenant context answer NOTHING (the store RLS
	// mirror of the 404 gate).
	t.Run("row-9 RLS blindness of the foreign tenant", func(t *testing.T) {
		caseID, workItemID, operation, _ := verificationCommittedOrder(t, w, svc, adapter, operator, "v9", fmtOrderTarget(909))
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		foreign := actionStore.NewPostgres(runtimePool)
		foreignScope, err := tenancy.New(id.MustParse("01994f64-6c00-7000-8000-00000000f009"), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		// The foreign PRINCIPAL of the SAME tenant reads nothing (the RLS
		// scope carries the tenant — the blindness rides the store's
		// tenant scoping of a DIFFERENT tenant id).
		otherTenant := id.MustParse("01994f64-6c00-7000-8000-00000000f010")
		otherScope, err := tenancy.New(id.MustParse("01994f64-6c00-7000-8000-00000000f011"), otherTenant)
		if err != nil {
			t.Fatal(err)
		}
		_ = foreignScope
		tasks, err := foreign.ReadCaseVerifications(ctx, otherScope, caseID)
		if err != nil {
			t.Fatalf("the foreign read degrades honestly, got %v", err)
		}
		if len(tasks) != 0 {
			t.Fatalf("the foreign tenant sees NO tasks, got %d", len(tasks))
		}
		events, err := foreign.ReadVerificationEvents(ctx, otherScope, caseID, 1)
		if err != nil || len(events) != 0 {
			t.Fatalf("the foreign tenant sees NO events, got %d err=%v", len(events), err)
		}
	})

	// Row 10 — the REPLAY ladder (AC-14): the decided operation answers
	// the repeated assignment with the NEW task verification_no=2 (the
	// repeated FULL check by the owner's decision — the history never
	// rewrites); the stale commands of the CLOSED task refuse with the
	// current state words.
	t.Run("row-10 replay: new task after the decision", func(t *testing.T) {
		caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, "v10", fmtOrderTarget(910))
		verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify4h, orderNow.Add(20*time.Minute))
		if _, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: orderNow.Add(22 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		// The repeated assignment of the SAME operation: the NEW task
		// (verification_no=2) — the ladder of the repeated full check.
		second := verificationAssign(t, svc, operator, operator, backup, w, caseID, workItemID, operation, actionDomain.Verify1d, orderNow.Add(30*time.Minute))
		if second.Task.VerificationNo != 2 {
			t.Fatalf("the ladder: the second task of the decided operation, got %d", second.Task.VerificationNo)
		}
		events := verificationEvents(t, w.tenant, caseID, 2)
		if len(events) != 1 || events[0].Kind != "assigned" {
			t.Fatalf("the second task opens with the assigned event, got %+v", events)
		}
		// The stale commands of the CLOSED first task refuse honestly.
		_, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, Now: orderNow.Add(31 * time.Minute),
		})
		if err == nil || !strings.Contains(err.Error(), "принятие") {
			t.Fatalf("the stale acknowledge of the closed task refuses, got %v", err)
		}
		_, err = svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
			ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: orderNow.Add(31 * time.Minute),
		})
		if err == nil || !strings.Contains(err.Error(), "решение возможно только из состояния") {
			t.Fatalf("the stale approve of the closed task refuses, got %v", err)
		}
		// The anchor refusal of the NON-COMMITTED operation: the assign
		// over an operation without the committed receipt answers the
		// honest anchor word.
		_, err = svc.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			OperationID: id.MustParse("01994f64-6c00-7000-8000-00000000f012"), Assignee: operator.ID,
			BackupOwner: backup.ID, Deadline: actionDomain.Verify4h, Now: orderNow.Add(32 * time.Minute),
		})
		if !errors.Is(err, actionDomain.ErrOrderVerificationNoReceipt) {
			t.Fatalf("the unknown operation anchor refuses, got %v", err)
		}
	})
}
