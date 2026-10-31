//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionCatalog "github.com/staffy-co/staffy/internal/action/catalog"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionPort "github.com/staffy-co/staffy/internal/action/port"
	actionStore "github.com/staffy-co/staffy/internal/action/store"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.8 (step-04): the order correction eval matrix — the AC-15 rows
// LIVE over the blocked-verification world of 4.7 (the Correction Work
// Item over the LAST CLOSED blocked decision). Every row owns its own
// case: the per-row 4-2..4-5 chain, the builder proposal, the
// confirmation, the authorization, the committed dispatch and the blocked
// decision — then the correction lifecycle walks the row's honest paths.
// ---------------------------------------------------------------------------

// correctionWorld wires the order world with the verification AND
// correction contours (the same service of the 4-6/4-7 matrices — the
// WithOrderCorrections wiring of buildOrderService).
func correctionWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, identityDomain.Principal, identityDomain.Principal) {
	t.Helper()
	w, svc, adapter, _, _, operator := orderWorld(t, seed)
	backup := identityDomain.Principal{ID: w.backup}
	return w, svc, adapter, operator, backup
}

// correctionBlockedOrder walks the full chain of one row's case to the
// BLOCKED verification decision (the correction anchor): the committed
// order receipt, the assigned-acknowledged task and the blocked decision
// with the affected position and the structured reason word.
func correctionBlockedOrder(t *testing.T, w *casesWorld, svc *actionApp.Service, adapter *countingMutationAdapter,
	operator, backup identityDomain.Principal, tag string, key string, reason string) (caseID, workItemID id.UUID, operation actionDomain.Operation) {
	t.Helper()
	ctx := context.Background()
	caseID, workItemID, operation, _ = verificationCommittedOrder(t, w, svc, adapter, operator, tag, key)
	if _, err := svc.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		OperationID: operation.OperationID, Assignee: operator.ID, BackupOwner: backup.ID,
		Deadline: actionDomain.Verify4h, Now: orderNow.Add(20 * time.Minute),
	}); err != nil {
		t.Fatalf("the assignment of the row %q refused: %v", tag, err)
	}
	if _, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
		ExpectedAssignment: 1, Now: orderNow.Add(21 * time.Minute),
	}); err != nil {
		t.Fatalf("the acceptance of the row %q refused: %v", tag, err)
	}
	blocked, err := svc.BlockOrderVerification(ctx, actionAPI.BlockOrderVerificationCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
		ExpectedAssignment: 1, Positions: []string{"WF2B196K666AA"}, Reason: reason,
		Now: orderNow.Add(22 * time.Minute),
	})
	if err != nil || blocked.View.State != actionDomain.OrderVerificationStateBlocked {
		t.Fatalf("the blocked decision of the row %q, got %+v err=%v", tag, blocked, err)
	}
	return caseID, workItemID, operation
}

// correctionOpen opens the correction of one row's blocked decision (the
// passing open of the matrix rows).
func correctionOpen(t *testing.T, svc *actionApp.Service, operator identityDomain.Principal, w *casesWorld,
	caseID id.UUID, operation actionDomain.Operation, deadline string, now time.Time) actionAPI.OrderCorrectionOutcome {
	t.Helper()
	outcome, err := svc.OpenOrderCorrection(context.Background(), actionAPI.OpenOrderCorrectionCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID,
		OperationID: operation.OperationID, Owner: operator.ID, Deadline: deadline, Now: now,
	})
	if err != nil {
		t.Fatalf("the open of the correction refused: %v", err)
	}
	return outcome
}

// correctionRecomposeConfirm walks the recompose path of one row: the
// builder re-runs over the same case (the EDIT ladder lands the preview
// version+1 of the SAME thread) and the customer confirms the NEW
// version (the confirmation thread version+1 with the superseded event
// on the old one). Returns the NEW confirmed version pins {proposal,
// version, confirmation} of the reread command.
func correctionRecomposeConfirm(t *testing.T, w *casesWorld, svc *actionApp.Service, operator identityDomain.Principal,
	caseID, workItemID id.UUID, tag string) (proposalID id.UUID, versionNo int64, confirmationID id.UUID, digest string) {
	t.Helper()
	ctx := context.Background()
	rebuilt, err := svc.PrepareOrderProposal(ctx, actionAPI.PrepareOrderProposalCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		Now: orderNow.Add(30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("the recompose of the row %q refused: %v", tag, err)
	}
	if rebuilt.Verdict != actionDomain.VerdictValid || rebuilt.VersionNo != 2 {
		t.Fatalf("the recomposed order proposal v2 of the row %q, got %+v", tag, rebuilt)
	}
	confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, operator, caseID, rebuilt.PreviewID, 2, 3600, orderNow.Add(31*time.Minute)))
	if err != nil {
		t.Fatalf("the confirmation of the new version of the row %q refused: %v", tag, err)
	}
	// The pinned digest of the new version (read from the LIVE preview
	// card — the fence material of the reread).
	thread, err := svc.ReadPreviewView(ctx, operator, w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, threadView := range thread.Threads {
		for _, card := range threadView.Cards {
			if card.PreviewID == rebuilt.PreviewID && card.VersionNo == 2 {
				digest = card.TypedInput["order_digest"]
			}
		}
	}
	if len(digest) != 64 {
		t.Fatalf("the pinned digest of the new version of the row %q, got %q", tag, digest)
	}
	return rebuilt.PreviewID, 2, confirmed.ConfirmationID, digest
}

// correctionEvents reads the events of one correction over the admin
// truth (the append-only rows — the SQL backstop of every lifecycle
// assertion).
func correctionEvents(t *testing.T, tenant, caseID id.UUID, correctionNo int64) []struct {
	EventNo   int64
	Kind      string
	Owner     string
	Deadline  string
	Positions string
	Reason    string
	External  string
	Proposal  string
	Version   int64
	Cessation string
} {
	t.Helper()
	rows, err := adminPool.Query(context.Background(),
		`select event_no, kind, coalesce(owner::text,''), coalesce(correct_deadline,''),
		        coalesce(positions::text,''), coalesce(reason,''),
		        coalesce(external_object_id,''), coalesce(proposal_id::text,''),
		        coalesce(version_no,0), coalesce(cessation_reason,'')
                 from action.order_correction_events
                 where tenant_id=$1 and case_id=$2 and correction_no=$3 order by event_no`,
		tenant, caseID, correctionNo)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type eventRow = struct {
		EventNo   int64
		Kind      string
		Owner     string
		Deadline  string
		Positions string
		Reason    string
		External  string
		Proposal  string
		Version   int64
		Cessation string
	}
	events := []eventRow{}
	for rows.Next() {
		var event eventRow
		if err := rows.Scan(&event.EventNo, &event.Kind, &event.Owner, &event.Deadline,
			&event.Positions, &event.Reason, &event.External, &event.Proposal,
			&event.Version, &event.Cessation); err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
	return events
}

// TestOrderCorrectionMatrixRowsBySpec walks the Story 4.8 eval matrix
// LIVE: the full correction chain row (blocked → open with the carried
// material BY NAME → the recompose and the customer confirmation of the
// new version → the reread with the SAFE pins → the corrected closure →
// the NEW verification task), the wrong-vehicle blocked reason carry, the
// price change DURING the correction (the composition fence refuses the
// reread, AC-13), the client refusal cessation with the contour-4-10
// recommendation (AC-11), the manual change contour probe (AG-4), the
// repeated open (the honest 409) and the replay refusal, the lost-link
// refusal, the superseded old Confirmation (AC-15 BY NAME), BOTH
// structural progression gates (the assign 409 and the authorization
// refusal BEFORE the transaction — nothing consumed, AC-2/AC-9), the RLS
// blindness of the foreign tenant and the supplier-send structural probe
// (OQ-3 — zero).
func TestOrderCorrectionMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	// The world seed 770: the free range 770-779 (the order family took
	// 755-759; the policy family owns 760-769 — the UNIQUE seed keeps the
	// shared-DB worlds collision-free, the 4-7 lesson of the seed 767
	// collision repeated by the first draft's 760).
	w, svc, adapter, operator, backup := correctionWorld(t, 770)

	// Row 1 — the FULL live chain: the blocked decision, the open with the
	// carried positions/reason BY NAME, the recompose and the customer
	// confirmation of the new version, the reread with the SAFE pins, the
	// corrected closure and the NEW verification task (the ladder
	// verification_no=2 — the former check never reuses).
	t.Run("row-1 full chain: blocked→open→recompose+confirm→reread→corrected→new task", func(t *testing.T) {
		caseID, workItemID, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c1", fmtOrderTarget(911), actionDomain.VerifyBlockReasonWrongPart)
		opened := correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		if opened.Correction.CorrectionNo != 1 || opened.Correction.OperationID != operation.OperationID {
			t.Fatalf("the correction 1 anchors the operation, got %+v", opened.Correction)
		}
		if opened.Correction.VerificationNo != 1 || opened.Correction.ExternalOrderID != "ABCP-ORDER-c1" {
			t.Fatalf("the blocked/receipt pins of the correction, got %+v", opened.Correction)
		}
		if opened.View.State != actionDomain.OrderCorrectionStatePending {
			t.Fatalf("the pending state, got %q", opened.View.State)
		}
		if opened.StateText != actionDomain.OrderCorrectionNotStartedWord {
			t.Fatalf("the «назначение ≠ начало работы» boundary word, got %q", opened.StateText)
		}
		// The SQL truth of the carry: the opened event carries the blocked
		// positions and the reason word BY NAME (the one-transaction
		// SQL-truth carry — the original blocked event never rewrites).
		events := correctionEvents(t, w.tenant, caseID, 1)
		if len(events) != 1 || events[0].Kind != "opened" {
			t.Fatalf("the opening event, got %+v", events)
		}
		if !strings.Contains(events[0].Positions, "WF2B196K666AA") || events[0].Reason != actionDomain.VerifyBlockReasonWrongPart {
			t.Fatalf("the carried blocked material BY NAME, got %q / %q", events[0].Positions, events[0].Reason)
		}
		if events[0].Owner != operator.ID.String() || events[0].Deadline != actionDomain.Correct4h {
			t.Fatalf("the round material of the opening, got %q / %q", events[0].Owner, events[0].Deadline)
		}
		// The original blocked event of the verification stays untouched
		// (the append-only truth of BOTH contours).
		verifyEvents := verificationEvents(t, w.tenant, caseID, 1)
		if len(verifyEvents) != 3 || verifyEvents[2].Kind != "blocked" {
			t.Fatalf("the original verification events stay untouched, got %+v", verifyEvents)
		}
		// The recompose and the customer confirmation of the new version.
		proposalID, versionNo, confirmationID, _ := correctionRecomposeConfirm(t, w, svc, operator, caseID, workItemID, "c1")
		// The reread with the NEW confirmed version pins (the manager's
		// words — AG-4): the corrected closure with the SAFE pin set.
		reread, err := svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c1-EDITED",
			ProposalID: proposalID, VersionNo: versionNo, ConfirmationID: confirmationID,
			Now: orderNow.Add(40 * time.Minute),
		})
		if err != nil {
			t.Fatalf("the reread refused: %v", err)
		}
		if reread.View.State != actionDomain.OrderCorrectionStateCorrected {
			t.Fatalf("the corrected state, got %q", reread.View.State)
		}
		if reread.StateText != actionDomain.OrderCorrectionRereadWord {
			t.Fatalf("the MANDATORY reread word (AG-4), got %q", reread.StateText)
		}
		if !strings.Contains(reread.Word, "pending_manager_verification") {
			t.Fatalf("the corrected word names the return to the manager verification, got %q", reread.Word)
		}
		// The SQL truth of the reread event: the SAFE pin set only (the
		// external object ID and the identity triple — NO blocked
		// material).
		events = correctionEvents(t, w.tenant, caseID, 1)
		if len(events) != 2 || events[1].Kind != "reread" {
			t.Fatalf("the reread event, got %+v", events)
		}
		if events[1].External != "ABCP-ORDER-c1-EDITED" || events[1].Proposal != proposalID.String() ||
			events[1].Version != 2 || events[1].Cessation != "" {
			t.Fatalf("the SAFE pins of the reread event, got %+v", events[1])
		}
		// The NEW verification task after the closure (the ladder
		// verification_no=2 — the assign gate no longer holds).
		second, err := svc.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			OperationID: operation.OperationID, Assignee: operator.ID, BackupOwner: backup.ID,
			Deadline: actionDomain.Verify1d, Now: orderNow.Add(50 * time.Minute),
		})
		if err != nil || second.Task.VerificationNo != 2 {
			t.Fatalf("the NEW verification task №2 after the correction closure, got %+v err=%v", second, err)
		}
	})

	// Row 2 — the WRONG VEHICLE blocked reason (AC-12 of 4-7 / the carry
	// of the correction): the blocked decision of the reason word
	// wrong_vehicle carries BY NAME into the opened event; the correction
	// opens over it with the same position pin.
	t.Run("row-2 wrong vehicle reason carries BY NAME", func(t *testing.T) {
		caseID, _, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c2", fmtOrderTarget(912), actionDomain.VerifyBlockReasonWrongVehicle)
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct1d, orderNow.Add(25*time.Minute))
		events := correctionEvents(t, w.tenant, caseID, 1)
		if len(events) != 1 || events[0].Reason != actionDomain.VerifyBlockReasonWrongVehicle {
			t.Fatalf("the wrong_vehicle reason carries BY NAME, got %+v", events)
		}
		if !strings.Contains(events[0].Positions, "WF2B196K666AA") {
			t.Fatalf("the position pin carries BY NAME, got %q", events[0].Positions)
		}
		if events[0].Deadline != actionDomain.Correct1d {
			t.Fatalf("the deadline word of the opening, got %q", events[0].Deadline)
		}
	})

	// Row 3 — the PRICE CHANGE during the correction (AC-13): the offer
	// re-record AFTER the customer confirmation of the new version moves
	// the LIVE digest — the reread is refused BY NAME with the
	// changed-composition word; the closure event is NEVER written (the
	// hidden degradation is impossible STRUCTURALLY).
	t.Run("row-3 price change during correction refuses the reread", func(t *testing.T) {
		caseID, workItemID, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c3", fmtOrderTarget(913), actionDomain.VerifyBlockReasonWrongPart)
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		proposalID, versionNo, confirmationID, _ := correctionRecomposeConfirm(t, w, svc, operator, caseID, workItemID, "c3")
		// The re-recorded offer: the CURRENT record of the supplier moves
		// the price (the live composition changes AFTER the confirmation).
		offerRecord(t, w, caseID, workItemID, "correct-stale", func(command *casesAPI.RecordSupplierOfferCommand) {
			command.SupplierID = "SUP-2"
			command.SupplierName = "Поставщик два"
			command.Brand = "Mann"
			command.ClientPrice = 4300
			command.PurchasePrice = nil
			command.QuantityAvailable = 6
			command.IdempotencyKey = "correct-stale-offer"
			command.SourceRecordID = "rec-correct-stale"
		})
		_, err := svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c3-EDITED",
			ProposalID: proposalID, VersionNo: versionNo, ConfirmationID: confirmationID,
			Now: orderNow.Add(40 * time.Minute),
		})
		if !errors.Is(err, actionDomain.ErrOrderCorrectionCompositionChanged) {
			t.Fatalf("the reread over the changed composition refuses BY NAME, got %v", err)
		}
		// The closure event is NEVER written — the correction stays open.
		events := correctionEvents(t, w.tenant, caseID, 1)
		if len(events) != 1 {
			t.Fatalf("the refused reread writes NO closure event, got %+v", events)
		}
		// The corrected closure stays reachable: the manager recomposes
		// again (v3) and confirms the updated composition — the reread
		// then passes the fence over the NEW confirmed version.
		rebuilt, err := svc.PrepareOrderProposal(ctx, actionAPI.PrepareOrderProposalCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			Now: orderNow.Add(45 * time.Minute),
		})
		if err != nil || rebuilt.VersionNo != 3 {
			t.Fatalf("the second recompose v3, got %+v err=%v", rebuilt, err)
		}
		confirmed, err := svc.ConfirmPreviewVersion(ctx, operationConfirmCommand(w, operator, caseID, rebuilt.PreviewID, 3, 3600, orderNow.Add(46*time.Minute)))
		if err != nil {
			t.Fatalf("the confirmation of v3 refused: %v", err)
		}
		if _, err = svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c3-EDITED",
			ProposalID: rebuilt.PreviewID, VersionNo: 3, ConfirmationID: confirmed.ConfirmationID,
			Now: orderNow.Add(47 * time.Minute),
		}); err != nil {
			t.Fatalf("the reread over the updated confirmed version passes the fence, got %v", err)
		}
	})

	// Row 4 — the CLIENT REFUSAL (AC-11): the ceased closure with the
	// structured reason word client_refused carries the cancellation
	// recommendation word (the contour 4-10 named); the absence of the
	// consent NEVER becomes a replacement (the correction closes WITHOUT
	// the reread material); the repeated cessation of the CLOSED
	// correction refuses honestly.
	t.Run("row-4 client refusal: ceased with the contour-4-10 word", func(t *testing.T) {
		caseID, workItemID, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c4", fmtOrderTarget(914), actionDomain.VerifyBlockReasonAvailabilityChanged)
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		ceased, err := svc.CeaseOrderCorrection(ctx, actionAPI.CeaseOrderCorrectionCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, Reason: actionDomain.CorrectionCessationClientRefused,
			Now: orderNow.Add(30 * time.Minute),
		})
		if err != nil {
			t.Fatalf("the cessation refused: %v", err)
		}
		if ceased.View.State != actionDomain.OrderCorrectionStateCeased {
			t.Fatalf("the ceased state, got %q", ceased.View.State)
		}
		if ceased.StateText != actionDomain.OrderCorrectionCancellationWord {
			t.Fatalf("the cancellation recommendation word of the client refusal, got %q", ceased.StateText)
		}
		if !strings.Contains(ceased.StateText, "контур 4-10") {
			t.Fatalf("the word names the contour 4-10, got %q", ceased.StateText)
		}
		// The SQL truth: the ceased event carries the reason word and NO
		// reread material.
		events := correctionEvents(t, w.tenant, caseID, 1)
		if len(events) != 2 || events[1].Kind != "ceased" || events[1].Cessation != actionDomain.CorrectionCessationClientRefused {
			t.Fatalf("the ceased event with the reason word, got %+v", events)
		}
		if events[1].External != "" || events[1].Proposal != "" || events[1].Version != 0 {
			t.Fatalf("the ceased event carries NO reread material, got %+v", events[1])
		}
		// The repeated cessation of the CLOSED correction refuses
		// honestly (the state word).
		_, err = svc.CeaseOrderCorrection(ctx, actionAPI.CeaseOrderCorrectionCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, Reason: actionDomain.CorrectionCessationNotNeeded,
			Now: orderNow.Add(31 * time.Minute),
		})
		if err == nil || !strings.Contains(err.Error(), "только из состояния") {
			t.Fatalf("the repeated cessation of the closed correction refuses, got %v", err)
		}
		// The NEW correction of the same operation opens ONLY by a NEW
		// blocked decision (the ladder correction_no stays one — the OLD
		// blocked decision was CONSUMED by the closed correction).
		_, err = svc.OpenOrderCorrection(ctx, actionAPI.OpenOrderCorrectionCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID,
			OperationID: operation.OperationID, Owner: operator.ID, Deadline: actionDomain.Correct4h,
			Now: orderNow.Add(32 * time.Minute),
		})
		if !errors.Is(err, actionDomain.ErrOrderCorrectionNoBlockedDecision) {
			t.Fatalf("the repeated open without a NEW blocked decision refuses the anchor word, got %v", err)
		}
		// The LADDER: the NEW blocked decision (the second verification
		// task blocks) opens the SECOND correction with the
		// verification_no=2 pin (the history never rewrites).
		if _, err = svc.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			OperationID: operation.OperationID, Assignee: operator.ID, BackupOwner: backup.ID,
			Deadline: actionDomain.Verify4h, Now: orderNow.Add(33 * time.Minute),
		}); err != nil {
			t.Fatalf("the second verification task after the ceased correction: %v", err)
		}
		if _, err = svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 2,
			ExpectedAssignment: 1, Now: orderNow.Add(34 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err = svc.BlockOrderVerification(ctx, actionAPI.BlockOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 2,
			ExpectedAssignment: 1, Positions: []string{"WF2B196K666AA"}, Reason: actionDomain.VerifyBlockReasonPriceChanged,
			Now: orderNow.Add(35 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		second := correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(36*time.Minute))
		if second.Correction.CorrectionNo != 2 || second.Correction.VerificationNo != 2 {
			t.Fatalf("the SECOND correction by the NEW blocked decision (the ladder), got %+v", second.Correction)
		}
		secondEvents := correctionEvents(t, w.tenant, caseID, 2)
		if len(secondEvents) != 1 || secondEvents[0].Reason != actionDomain.VerifyBlockReasonPriceChanged {
			t.Fatalf("the second correction carries the NEW blocked material, got %+v", secondEvents)
		}
	})

	// Row 5 — the MANUAL CHANGE contour (AG-4/AC-10): the ABCP order
	// change NEVER happens by a Staffy action — the catalog carries NO
	// change command (the structural probe), the mutation adapter stays
	// at ZERO calls through the whole correction lifecycle, and the
	// mandatory manual-contour word rides the domain.
	t.Run("row-5 manual change contour: no change command, adapter zero", func(t *testing.T) {
		caseID, _, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c5", fmtOrderTarget(915), actionDomain.VerifyBlockReasonConfirmationIssue)
		adapter.reset()
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		if _, err := svc.CeaseOrderCorrection(ctx, actionAPI.CeaseOrderCorrectionCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, Reason: actionDomain.CorrectionCessationAbandoned,
			Now: orderNow.Add(30 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		if adapter.total() != 0 {
			t.Fatalf("the correction lifecycle calls NO adapter (AG-4), adapter calls %d", adapter.total())
		}
		// The IN-CODE structural probe (AG-4: the catalog is the BINARY
		// list — the production composition root passes it in code): NO
		// change/update/modify command exists in the product catalog.
		for _, def := range actionCatalog.All() {
			id := strings.ToLower(def.ID)
			if strings.Contains(id, "change") || strings.Contains(id, "update") || strings.Contains(id, "modify") {
				t.Fatalf("the catalog carries a change command (AG-4 fence): %s", def.ID)
			}
		}
		// The domain probe: the manual-contour word is the honest
		// negation of the change (three parts).
		for _, needle := range []string{"вручную в контуре магазина", "Staffy не изменяет", "не создаёт новый заказ автоматически"} {
			if !strings.Contains(actionDomain.OrderCorrectionManualContourWord, needle) {
				t.Fatalf("the manual-contour word lost the part %q: %q", needle, actionDomain.OrderCorrectionManualContourWord)
			}
		}
	})

	// Row 6 — the REPEATED command (the honest 409 and the replay): the
	// second open of the SAME operation while the correction lives
	// refuses with the active conflict (the current state words travel);
	// the reread of the CLOSED correction refuses with the current state
	// word (the replay never writes).
	t.Run("row-6 repeated open 409, replay of the closed refusal", func(t *testing.T) {
		caseID, workItemID, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c6", fmtOrderTarget(916), actionDomain.VerifyBlockReasonWrongPart)
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		_, err := svc.OpenOrderCorrection(ctx, actionAPI.OpenOrderCorrectionCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID,
			OperationID: operation.OperationID, Owner: operator.ID, Deadline: actionDomain.Correct4h,
			Now: orderNow.Add(26 * time.Minute),
		})
		var active actionDomain.OrderCorrectionActiveError
		if !errors.As(err, &active) {
			t.Fatalf("the repeated open refuses the active 409, got %v", err)
		}
		if active.State != actionDomain.OrderCorrectionStatePending || active.StateText != actionDomain.OrderCorrectionStateText(actionDomain.OrderCorrectionStatePending) {
			t.Fatalf("the current state words travel with the conflict, got %+v", active)
		}
		// The closed correction: the reread replay refuses with the
		// current state word (nothing writes).
		proposalID, versionNo, confirmationID, _ := correctionRecomposeConfirm(t, w, svc, operator, caseID, workItemID, "c6")
		if _, err = svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c6-EDITED",
			ProposalID: proposalID, VersionNo: versionNo, ConfirmationID: confirmationID,
			Now: orderNow.Add(40 * time.Minute),
		}); err != nil {
			t.Fatalf("the first reread passes: %v", err)
		}
		_, err = svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c6-EDITED-2",
			ProposalID: proposalID, VersionNo: versionNo, ConfirmationID: confirmationID,
			Now: orderNow.Add(41 * time.Minute),
		})
		if err == nil || !strings.Contains(err.Error(), "только из состояния") {
			t.Fatalf("the reread replay of the closed correction refuses, got %v", err)
		}
		events := correctionEvents(t, w.tenant, caseID, 1)
		if len(events) != 2 {
			t.Fatalf("the replay writes NO second closure, got %+v", events)
		}
	})

	// Row 7 — the LOST LINK (the reread without a correction): the reread
	// of a NONEXISTENT correction answers the honest 404 (the RLS
	// blindness mirror of the store); the foreign triple pin refuses the
	// resolve.
	t.Run("row-7 lost link: reread without a correction refuses", func(t *testing.T) {
		caseID, _, _ := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c7", fmtOrderTarget(917), actionDomain.VerifyBlockReasonWrongPart)
		_, err := svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 7,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c7-EDITED",
			ProposalID: id.MustParse("01994f64-6c00-7000-8000-00000000f071"), VersionNo: 1,
			ConfirmationID: id.MustParse("01994f64-6c00-7000-8000-00000000f072"),
			Now:            orderNow.Add(30 * time.Minute),
		})
		if !errors.Is(err, actionPort.ErrCorrectionNotFound) && !strings.Contains(err.Error(), "не найдена") && !errors.Is(err, actionDomain.ErrNotFound) {
			t.Fatalf("the reread of the missing correction refuses the 404, got %v", err)
		}
	})

	// Row 8 — the OLD CONFIRMATION (AC-15): the reread over the
	// SUPERSEDED confirmation version (the original order's consent —
	// the recompose superseded it by the new version) refuses BY NAME
	// with the superseded word; the closure event is NEVER written.
	t.Run("row-8 superseded old confirmation refuses BY NAME", func(t *testing.T) {
		caseID, workItemID, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c8", fmtOrderTarget(918), actionDomain.VerifyBlockReasonWrongPart)
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		// The original confirmation of the first order (v1 of the thread):
		// the recompose lands v2 — the v1 superseded event rides the SAME
		// transaction of the new confirmation.
		thread, err := svc.ReadPreviewView(ctx, operator, w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		var oldProposalID id.UUID
		var oldDigest string
		for _, threadView := range thread.Threads {
			for _, card := range threadView.Cards {
				if card.DefinitionID == actionDomain.OrderActionID && card.VersionNo == 1 {
					oldProposalID = card.PreviewID
					oldDigest = card.TypedInput["order_digest"]
				}
			}
		}
		if !oldProposalID.Valid() || oldDigest == "" {
			t.Fatal("the original order preview v1 of the row")
		}
		// The original confirmation thread v1 (the confirmation of the
		// FIRST order — the authorize consumed it; the recompose supersedes
		// the version by the v2 confirmation of the SAME thread).
		correctionRecomposeConfirm(t, w, svc, operator, caseID, workItemID, "c8")
		// The OLD triple {proposal, version 1, the original confirmation}
		// pins the superseded version — the reread refuses BY NAME. The
		// original confirmation id rides the OPERATION row itself (the
		// anchor of the first authorization — the v1 consent of the
		// thread).
		oldConfirmationID := operation.ConfirmationID
		if !oldConfirmationID.Valid() {
			t.Fatal("the original confirmation v1 of the row")
		}
		_, err = svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c8-EDITED",
			ProposalID: oldProposalID, VersionNo: 1, ConfirmationID: oldConfirmationID,
			Now: orderNow.Add(40 * time.Minute),
		})
		if !errors.Is(err, actionDomain.ErrOrderCorrectionSupersededConfirmation) {
			t.Fatalf("the old superseded confirmation refuses BY NAME (AC-15), got %v", err)
		}
		// The closure event is NEVER written — the correction stays open
		// (the reread with the NEW version still passes).
		events := correctionEvents(t, w.tenant, caseID, 1)
		if len(events) != 1 {
			t.Fatalf("the superseded reread writes NO closure, got %+v", events)
		}
	})

	// Row 9 — BOTH structural gates (AC-2/AC-9): the assign of the new
	// verification task while the correction is open refuses with the
	// 409 gate word; the authorization of the NEW order while the
	// correction is open refuses BEFORE the reservation transaction —
	// NOTHING is consumed and NO operation row lands; after the closure
	// both paths open again.
	t.Run("row-9 both gates: assign 409 and authorization before the transaction", func(t *testing.T) {
		caseID, workItemID, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c9", fmtOrderTarget(919), actionDomain.VerifyBlockReasonWrongPart)
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		// The assign gate: the new verification task NEVER exists while
		// the correction lives.
		_, err := svc.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			OperationID: operation.OperationID, Assignee: operator.ID, BackupOwner: backup.ID,
			Deadline: actionDomain.Verify4h, Now: orderNow.Add(26 * time.Minute),
		})
		var gate actionDomain.OrderCorrectionAssignGateError
		if !errors.As(err, &gate) || err.Error() != actionDomain.OrderCorrectionAssignGateWord {
			t.Fatalf("the assign gate refuses with the gate word, got %v", err)
		}
		// The authorization gate: the recomposed+confirmed version author
		// a NEW order while the correction is open — the refusal lands
		// BEFORE the transaction.
		proposalID, versionNo, confirmationID, _ := correctionRecomposeConfirm(t, w, svc, operator, caseID, workItemID, "c9")
		authorized, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, operator, caseID, confirmationID, versionNo, fmtOrderTarget(919)+"-new", orderNow.Add(35*time.Minute)))
		if !errors.Is(err, actionDomain.ErrOrderCorrectionOpenGate) {
			t.Fatalf("the authorization gate refuses BEFORE the transaction, got %+v err=%v", authorized, err)
		}
		if err.Error() != actionDomain.OrderCorrectionOrderGateWord {
			t.Fatalf("the gate word of the authorization, got %q", err.Error())
		}
		// NOTHING was consumed and NO new operation row landed (the
		// consumption count of the new confirmation stays zero).
		if count := operationConsumedCount(t, w.tenant, confirmationID, versionNo); count != 0 {
			t.Fatalf("the gate consumes NOTHING, got %d", count)
		}
		view, err := svc.ReadAuthorizationView(ctx, operator, w.tenant, caseID, orderNow.Add(36*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		for _, card := range view.Operations {
			if card.IdempotencyKey == fmtOrderTarget(919)+"-new" {
				t.Fatal("the gate lands NO new operation row")
			}
		}
		_ = proposalID
		// After the closure both paths open: the assign passes (the NEW
		// task) and the authorization proceeds past the gate.
		if _, err = svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-c9-EDITED",
			ProposalID: proposalID, VersionNo: versionNo, ConfirmationID: confirmationID,
			Now: orderNow.Add(40 * time.Minute),
		}); err != nil {
			t.Fatalf("the reread passes after the fence: %v", err)
		}
		if _, err = svc.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			OperationID: operation.OperationID, Assignee: operator.ID, BackupOwner: backup.ID,
			Deadline: actionDomain.Verify4h, Now: orderNow.Add(41 * time.Minute),
		}); err != nil {
			t.Fatalf("the assign passes after the closure, got %v", err)
		}
		past, err := svc.AuthorizeOperation(ctx, operationAuthorizeCommand(w, operator, caseID, confirmationID, versionNo, fmtOrderTarget(919)+"-after", orderNow.Add(42*time.Minute)))
		if errors.Is(err, actionDomain.ErrOrderCorrectionOpenGate) {
			t.Fatalf("the gate no longer holds after the closure, got %v", err)
		}
		// The honest later refusals of the world's own machine are NEVER
		// the gate (the replay/idempotency words ride the 3-5 ladder).
		_ = past
	})

	// Row 10 — the RLS BLINDNESS and the SUPPLIER-SEND probe (OQ-3): the
	// correction reads of a foreign tenant context answer NOTHING (the
	// store RLS mirror); NO supplier-send column exists on the correction
	// tables and the honest words that mention the send are the
	// NEGATIONS only.
	t.Run("row-10 RLS blindness and supplier-send probe zero", func(t *testing.T) {
		caseID, _, operation := correctionBlockedOrder(t, w, svc, adapter, operator, backup, "c10", fmtOrderTarget(920), actionDomain.VerifyBlockReasonWrongPart)
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(25*time.Minute))
		foreign := actionStore.NewPostgres(runtimePool)
		otherTenant := id.MustParse("01994f64-6c00-7000-8000-00000000f020")
		otherScope, err := tenancy.New(id.MustParse("01994f64-6c00-7000-8000-00000000f021"), otherTenant)
		if err != nil {
			t.Fatal(err)
		}
		corrections, err := foreign.ReadCaseCorrections(ctx, otherScope, caseID)
		if err != nil || len(corrections) != 0 {
			t.Fatalf("the foreign tenant sees NO corrections, got %d err=%v", len(corrections), err)
		}
		events, err := foreign.ReadCorrectionEvents(ctx, otherScope, caseID, 1)
		if err != nil || len(events) != 0 {
			t.Fatalf("the foreign tenant sees NO correction events, got %d err=%v", len(events), err)
		}
		// The SQL structural probe: no send/supplier column exists on the
		// correction tables.
		rows, err := adminPool.Query(ctx,
			`select column_name from information_schema.columns
                         where table_schema='action' and table_name in ('order_corrections','order_correction_events')
                         and (column_name ilike '%send%' or column_name ilike '%supplier%')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if rows.Next() {
			t.Fatal("the correction tables carry a supplier-send column")
		}
		// The domain probe: the send-mentioning words are the honest
		// negations only («прежде не отправлялся»).
		if !strings.Contains(actionDomain.OrderCorrectionNotSentWord, "не отправлялся") {
			t.Fatalf("the not-sent word is the honest negation: %q", actionDomain.OrderCorrectionNotSentWord)
		}
	})
}
