//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	actionAPI "github.com/staffy-co/staffy/internal/action/api"
	actionApp "github.com/staffy-co/staffy/internal/action/app"
	actionDomain "github.com/staffy-co/staffy/internal/action/domain"
	actionStore "github.com/staffy-co/staffy/internal/action/store"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// ---------------------------------------------------------------------------
// Story 4.9 (step-04): the order status eval matrix — the AC-15 rows
// LIVE over the committed-order world of 4-6/4-7 (the Order Status
// Projection over the material of the three machines plus the Supplier
// Order Fact Receipt). Every row owns its own case: the per-row
// 4-2..4-5 chain, the builder proposal, the confirmation, the
// authorization, the committed dispatch and the verification decision —
// then the fact ladder and the projection walk the row's honest paths.
// ---------------------------------------------------------------------------

// statusWorld wires the order world with the status contour (the same
// service of the 4-6/4-7/4-8 matrices — the WithOrderSupplierFacts
// wiring of buildOrderService).
func statusWorld(t *testing.T, seed int) (*casesWorld, *actionApp.Service, *countingMutationAdapter, identityDomain.Principal, identityDomain.Principal) {
	t.Helper()
	w, svc, adapter, _, _, operator := orderWorld(t, seed)
	backup := identityDomain.Principal{ID: w.backup}
	return w, svc, adapter, operator, backup
}

// statusApprovedOrder walks the full chain of one row's case to the
// APPROVED verification decision: the committed order receipt, the
// assigned-acknowledged task and the approved decision with the digest
// pin — the anchor state of the fact ladder rows.
func statusApprovedOrder(t *testing.T, w *casesWorld, svc *actionApp.Service, adapter *countingMutationAdapter,
	operator, backup identityDomain.Principal, tag string, key string) (caseID, workItemID id.UUID, operation actionDomain.Operation) {
	t.Helper()
	ctx := context.Background()
	caseID, workItemID, operation, digest := verificationCommittedOrder(t, w, svc, adapter, operator, tag, key)
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
	if _, err := svc.ApproveOrderVerification(ctx, actionAPI.ApproveOrderVerificationCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 1,
		ExpectedAssignment: 1, ExpectedOrderDigest: digest, Now: orderNow.Add(22 * time.Minute),
	}); err != nil {
		t.Fatalf("the approval of the row %q refused: %v", tag, err)
	}
	return caseID, workItemID, operation
}

// statusRecordFact runs the passing record command of one row (the kind
// word decides the command).
func statusRecordFact(t *testing.T, svc *actionApp.Service, operator identityDomain.Principal, w *casesWorld,
	caseID id.UUID, operation actionDomain.Operation, kind, supplierOrderID string, happened time.Time, now time.Time) actionAPI.OrderSupplierFactOutcome {
	t.Helper()
	command := actionAPI.RecordOrderSupplierFactCommand{
		Actor: operator, TenantID: w.tenant, CaseID: caseID,
		OperationID: operation.OperationID, SupplierOrderID: supplierOrderID,
		HappenedAt: happened, Now: now,
	}
	var outcome actionAPI.OrderSupplierFactOutcome
	var err error
	if kind == actionDomain.OrderSupplierFactSent {
		outcome, err = svc.RecordOrderSupplierSent(context.Background(), command)
	} else {
		outcome, err = svc.RecordOrderSupplierFormed(context.Background(), command)
	}
	if err != nil {
		t.Fatalf("the record of the %s fact %q refused: %v", kind, supplierOrderID, err)
	}
	return outcome
}

// statusView reads the projection of one row's case.
func statusView(t *testing.T, svc *actionApp.Service, operator identityDomain.Principal, w *casesWorld, caseID id.UUID) actionDomain.OrderStatusProjection {
	t.Helper()
	projection, err := svc.ReadOrderStatusView(context.Background(), operator, w.tenant, caseID, orderNow.Add(50*time.Minute))
	if err != nil {
		t.Fatalf("the projection read of the case %s refused: %v", caseID, err)
	}
	return projection
}

// statusFactRows reads the fact rows of one case over the admin truth
// (the append-only rows — the SQL backstop of every ladder assertion).
func statusFactRows(t *testing.T, tenant, caseID id.UUID) []struct {
	FactNo   int64
	Kind     string
	Supplier string
} {
	t.Helper()
	rows, err := adminPool.Query(context.Background(),
		`select fact_no, kind, supplier_order_id from action.order_supplier_facts
                 where tenant_id=$1 and case_id=$2 order by fact_no`,
		tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	facts := []struct {
		FactNo   int64
		Kind     string
		Supplier string
	}{}
	for rows.Next() {
		var fact struct {
			FactNo   int64
			Kind     string
			Supplier string
		}
		if err := rows.Scan(&fact.FactNo, &fact.Kind, &fact.Supplier); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, fact)
	}
	return facts
}

// TestOrderStatusMatrixRowsBySpec walks the Story 4.9 eval matrix LIVE:
// the manual-send row (the records confirm the stages — BEFORE the
// record the stage is NEVER claimed), the missing supplier receipt
// (AC-4 — the honest «не утверждается» words), the webhook repeat (the
// idempotent replay BY KEY — AC-9), the out-of-order events (the
// records never roll back — AC-9), the system conflict (the divergent
// external IDs BOTH shown — AC-8), the partial send (the honest
// aggregate — AC-15), the ABCP→1C delay (1C stays unknown — OQ-4), the
// stale status (the freshness without the schedule — AC-10), the ladder
// refusal, the RLS blindness and the supplier-send structural probe
// (OQ-3 — zero).
func TestOrderStatusMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	// The world seed 771: the free range of the order family (755-759
	// and 770 are taken — the UNIQUE seed keeps the shared-DB worlds
	// collision-free, the 4-7/4-8 lessons of the seed register).
	w, svc, adapter, operator, backup := statusWorld(t, 771)

	// Row 1 — the MANUAL SEND: the full chain with the approved
	// verification, then the formed and sent records of the SAME
	// supplier order ID — the stages confirm with the manager_record
	// source; BEFORE the records the supplier stages are NEVER claimed
	// (the false send claim is impossible structurally, AC-15).
	t.Run("row-1 manual send: formed→sent records confirm the stages; before the record nothing is claimed", func(t *testing.T) {
		caseID, workItemID, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "s1", fmtOrderTarget(931))
		_ = workItemID
		// BEFORE the records: the supplier stages stay unconfirmed with
		// the honest «не утверждается» words (AC-4/AC-15).
		early := statusView(t, svc, operator, w, caseID)
		if early.Stages[3].Confirmed || early.Stages[4].Confirmed {
			t.Fatalf("the supplier stages are claimed BEFORE the records: %+v", early.Stages)
		}
		if !strings.Contains(early.Stages[4].Word, "утверждение об отправке не делается") {
			t.Fatalf("the unclaimed send word: %q", early.Stages[4].Word)
		}
		if early.CurrentStage != actionDomain.OrderStageManagerVerified {
			t.Fatalf("the current stage before the records: %q", early.CurrentStage)
		}
		// The formed record: the fact №1 with the TWO times (the named
		// happened_at and the server recorded_at never mix).
		formed := statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-31",
			orderNow.Add(30*time.Minute), orderNow.Add(31*time.Minute))
		if formed.Replay || formed.Fact.FactNo != 1 || formed.SourceWord != actionDomain.OrderStatusSupplierManagerSourceWord {
			t.Fatalf("the formed record: %+v", formed)
		}
		if !formed.Fact.HappenedAt.Equal(orderNow.Add(30*time.Minute)) || !formed.Fact.RecordedAt.Equal(orderNow.Add(31*time.Minute)) {
			t.Fatalf("the two times of the formed fact: %v / %v", formed.Fact.HappenedAt, formed.Fact.RecordedAt)
		}
		// The sent record of the SAME supplier order ID: the fact №2 —
		// the honest aggregate confirms the sent stage.
		sent := statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactSent, "SUP-31",
			orderNow.Add(40*time.Minute), orderNow.Add(41*time.Minute))
		if sent.Replay || sent.Fact.FactNo != 2 {
			t.Fatalf("the sent record: %+v", sent)
		}
		projection := statusView(t, svc, operator, w, caseID)
		if !projection.Stages[3].Confirmed || !projection.Stages[4].Confirmed {
			t.Fatalf("the supplier stages after the records: %+v", projection.Stages)
		}
		for _, stage := range []actionDomain.OrderStageEvidence{projection.Stages[3], projection.Stages[4]} {
			if stage.Source != actionDomain.OrderFactSourceManagerRecord {
				t.Fatalf("the supplier stage source: %q", stage.Source)
			}
		}
		if projection.CurrentStage != actionDomain.OrderStageSupplierSent {
			t.Fatalf("the current stage after the records: %q", projection.CurrentStage)
		}
		// The ABCP→1C delay row rides the SAME projection (the 1C block
		// stays unknown after EVERYTHING — OQ-4: the creation is never a
		// synchronization).
		if !strings.Contains(projection.Systems[1].Word, "unknown") {
			t.Fatalf("the 1C word after the full chain: %q", projection.Systems[1].Word)
		}
		// The per-system block: ABCP carries the receipt fact with the
		// system_receipt source; the supplier block carries BOTH facts
		// with the manager_record source (AC-5).
		if projection.Systems[0].Facts[0].Source != actionDomain.OrderFactSourceSystemReceipt {
			t.Fatalf("the ABCP fact source: %q", projection.Systems[0].Facts[0].Source)
		}
		if len(projection.Systems[2].Facts) != 2 {
			t.Fatalf("the supplier facts: %d", len(projection.Systems[2].Facts))
		}
		// The deadline block: the actual event recorded with the time
		// (AC-11); the freshness carries the last confirmed fact (AC-10)
		// with the no-schedule word.
		if !projection.Deadline.ActualRecorded {
			t.Fatal("the actual deadline event is not recorded after the sent record")
		}
		if !projection.Freshness.LastConfirmedAt.Equal(orderNow.Add(41 * time.Minute)) {
			t.Fatalf("the last confirmed fact time: %v", projection.Freshness.LastConfirmedAt)
		}
		if !strings.Contains(projection.Freshness.NoScheduleWord, "расписание не существует") {
			t.Fatalf("the no-schedule word: %q", projection.Freshness.NoScheduleWord)
		}
		// The SQL backstop: exactly TWO fact rows with the ladder
		// numbers and the kind words.
		rows := statusFactRows(t, w.tenant, caseID)
		if len(rows) != 2 || rows[0].Kind != actionDomain.OrderSupplierFactFormed || rows[1].Kind != actionDomain.OrderSupplierFactSent {
			t.Fatalf("the fact rows of the full chain: %+v", rows)
		}
	})

	// Row 2 — the WEBHOOK REPEAT (AC-9): the repeated formed command of
	// the SAME key answers the SAVED fact with the replay flag — NO
	// second row lands; the formed fact of ANOTHER supplier order ID is
	// the SECOND fact (the multiple suppliers live by name).
	t.Run("row-2 webhook repeat: the replay BY KEY, never a second row", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "s2", fmtOrderTarget(932))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-32",
			orderNow.Add(30*time.Minute), orderNow.Add(31*time.Minute))
		replay := statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-32",
			orderNow.Add(45*time.Minute), orderNow.Add(46*time.Minute))
		if !replay.Replay || replay.Fact.FactNo != 1 {
			t.Fatalf("the replay outcome: %+v", replay)
		}
		// The first write wins: the replayed fact carries the ORIGINAL
		// times (the history is never rewritten).
		if !replay.Fact.HappenedAt.Equal(orderNow.Add(30 * time.Minute)) {
			t.Fatalf("the replay carries the original happened_at: %v", replay.Fact.HappenedAt)
		}
		if got := statusFactRows(t, w.tenant, caseID); len(got) != 1 {
			t.Fatalf("the fact rows after the repeat: %+v", got)
		}
		// The formed fact of ANOTHER supplier order ID: the SECOND fact.
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-33",
			orderNow.Add(35*time.Minute), orderNow.Add(36*time.Minute))
		if got := statusFactRows(t, w.tenant, caseID); len(got) != 2 {
			t.Fatalf("the fact rows after the second supplier: %+v", got)
		}
	})

	// Row 3 — the OUT-OF-ORDER EVENTS (AC-9): the sent record names an
	// EARLIER happened_at than the formed record (the manager recorded
	// the events out of order — the truth is recorded as it is); BOTH
	// stages still confirm (the records never roll back), the timeline
	// orders by the display time (the sent lands BEFORE the formed —
	// the honest chronology).
	t.Run("row-3 out-of-order events: the same result, the display chronology honest", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "s3", fmtOrderTarget(933))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-34",
			orderNow.Add(40*time.Minute), orderNow.Add(41*time.Minute))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactSent, "SUP-34",
			orderNow.Add(30*time.Minute), orderNow.Add(42*time.Minute))
		projection := statusView(t, svc, operator, w, caseID)
		if !projection.Stages[3].Confirmed || !projection.Stages[4].Confirmed {
			t.Fatalf("the out-of-order records still confirm the stages: %+v", projection.Stages)
		}
		if projection.CurrentStage != actionDomain.OrderStageSupplierSent {
			t.Fatalf("the current stage: %q", projection.CurrentStage)
		}
		// The timeline display: the sent (happened earlier) renders
		// BEFORE the formed — the chronology of the NAMED times.
		sentBefore := false
		for i, entry := range projection.Timeline {
			if entry.Kind == "supplier_fact_sent" {
				for j, other := range projection.Timeline {
					if other.Kind == "supplier_fact_formed" && i < j {
						sentBefore = true
					}
				}
			}
		}
		if !sentBefore {
			t.Fatalf("the timeline does not order by the named times: %+v", projection.Timeline)
		}
	})

	// Row 4 — the SYSTEM CONFLICT (AC-8): the reread external ID of the
	// correction 4-8 diverges from the receipt external ID — the ABCP
	// block carries BOTH facts with their OWN sources, the divergence
	// word names the refusal of the favorable choice.
	t.Run("row-4 system conflict: both external IDs separately, the favorable version never chosen", func(t *testing.T) {
		caseID, workItemID, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "s4", fmtOrderTarget(934))
		// The blocked decision → the correction → the reread with the
		// DIFFERENT external object ID (the walk of the 4-8 helpers).
		if _, err := svc.AssignOrderVerification(ctx, actionAPI.AssignOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			OperationID: operation.OperationID, Assignee: operator.ID, BackupOwner: backup.ID,
			Deadline: actionDomain.Verify4h, Now: orderNow.Add(23 * time.Minute),
		}); err != nil {
			t.Fatalf("the second assignment refused: %v", err)
		}
		// The ladder: the NEW task (№2) after the approved one.
		if _, err := svc.AcknowledgeOrderVerification(ctx, actionAPI.AcknowledgeOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 2,
			ExpectedAssignment: 1, Now: orderNow.Add(24 * time.Minute),
		}); err != nil {
			t.Fatalf("the acceptance of the second task refused: %v", err)
		}
		blocked, err := svc.BlockOrderVerification(ctx, actionAPI.BlockOrderVerificationCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, VerificationNo: 2,
			ExpectedAssignment: 1, Positions: []string{"WF2B196K666AA"}, Reason: actionDomain.VerifyBlockReasonWrongPart,
			Now: orderNow.Add(25 * time.Minute),
		})
		if err != nil || blocked.View.State != actionDomain.OrderVerificationStateBlocked {
			t.Fatalf("the blocked decision, got %+v err=%v", blocked, err)
		}
		correctionOpen(t, svc, operator, w, caseID, operation, actionDomain.Correct4h, orderNow.Add(26*time.Minute))
		proposalID, versionNo, confirmationID, _ := correctionRecomposeConfirm(t, w, svc, operator, caseID, workItemID, "s4")
		if _, err := svc.RecordOrderCorrectionReread(ctx, actionAPI.RecordOrderCorrectionRereadCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID, CorrectionNo: 1,
			ExpectedAssignment: 1, ExternalObjectID: "ABCP-ORDER-CONFLICT",
			ProposalID: proposalID, VersionNo: versionNo, ConfirmationID: confirmationID,
			Now: orderNow.Add(40 * time.Minute),
		}); err != nil {
			t.Fatalf("the reread with the divergent ID refused: %v", err)
		}
		projection := statusView(t, svc, operator, w, caseID)
		abcp := projection.Systems[0]
		if !abcp.DivergentIDs || len(abcp.Facts) != 2 {
			t.Fatalf("the divergent ABCP block: %+v", abcp)
		}
		sources := map[string]string{}
		for _, fact := range abcp.Facts {
			sources[fact.ExternalID] = fact.Source
		}
		if len(sources) != 2 {
			t.Fatalf("the divergent facts are not both present: %+v", abcp.Facts)
		}
		for _, fact := range abcp.Facts {
			if fact.ExternalID == "ABCP-ORDER-CONFLICT" && fact.Source != actionDomain.OrderFactSourceManagerRecord {
				t.Fatalf("the reread fact source: %q", fact.Source)
			}
		}
		if !strings.Contains(abcp.Word, actionDomain.OrderStatusExternalIDDivergenceWord) {
			t.Fatalf("the divergence word: %q", abcp.Word)
		}
	})

	// Row 5 — the PARTIAL SEND (AC-15): two formed facts with ONE sent
	// record — the sent stage stays UNCONFIRMED with the by-name words;
	// the mixed stages NEVER claim the full send (the structural probe:
	// the derivation without the full coverage NEVER yields
	// supplier_sent).
	t.Run("row-5 partial send: the honest aggregate, never a full-send claim", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "s5", fmtOrderTarget(935))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-51",
			orderNow.Add(30*time.Minute), orderNow.Add(31*time.Minute))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-52",
			orderNow.Add(32*time.Minute), orderNow.Add(33*time.Minute))
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactSent, "SUP-51",
			orderNow.Add(40*time.Minute), orderNow.Add(41*time.Minute))
		projection := statusView(t, svc, operator, w, caseID)
		if projection.Stages[4].Confirmed {
			t.Fatal("the partial send confirms the sent stage — the false claim")
		}
		if !strings.Contains(projection.Stages[4].Word, "отправлен частично") {
			t.Fatalf("the partial-send word: %q", projection.Stages[4].Word)
		}
		if !strings.Contains(projection.Stages[4].Detail, "SUP-51 отправлен") ||
			!strings.Contains(projection.Stages[4].Detail, "SUP-52 сформирован и не записан как отправленный") {
			t.Fatalf("the by-name partial detail: %q", projection.Stages[4].Detail)
		}
		if projection.CurrentStage != actionDomain.OrderStageSupplierFormed {
			t.Fatalf("the current stage of the partial send: %q", projection.CurrentStage)
		}
		if !strings.Contains(projection.NextStep, "SUP-52") {
			t.Fatalf("the next step names the uncovered formation: %q", projection.NextStep)
		}
	})

	// Row 6 — the LADDER REFUSAL: the sent record without the prior
	// formed record of the SAME supplier order ID refuses with the
	// ladder word; the sent of ANOTHER ID does not open the path.
	t.Run("row-6 ladder refusal: sent only after formed of the same ID", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "s6", fmtOrderTarget(936))
		command := actionAPI.RecordOrderSupplierFactCommand{
			Actor: operator, TenantID: w.tenant, CaseID: caseID,
			OperationID: operation.OperationID, SupplierOrderID: "SUP-61",
			HappenedAt: orderNow.Add(30 * time.Minute), Now: orderNow.Add(31 * time.Minute),
		}
		_, err := svc.RecordOrderSupplierSent(ctx, command)
		if err == nil || !strings.Contains(err.Error(), "сначала запишите формирование") {
			t.Fatalf("the ladder refusal word, got %v", err)
		}
		// The formed fact of ANOTHER ID does not open the sent path.
		statusRecordFact(t, svc, operator, w, caseID, operation, actionDomain.OrderSupplierFactFormed, "SUP-62",
			orderNow.Add(32*time.Minute), orderNow.Add(33*time.Minute))
		_, err = svc.RecordOrderSupplierSent(ctx, command)
		if err == nil || !strings.Contains(err.Error(), "сначала запишите формирование") {
			t.Fatalf("the ladder refusal of the other ID, got %v", err)
		}
		if got := statusFactRows(t, w.tenant, caseID); len(got) != 1 {
			t.Fatalf("the fact rows after the refusals: %+v", got)
		}
	})

	// Row 7 — the STALE STATUS (AC-10): the projection of a case
	// WITHOUT any material movement carries the empty freshness time
	// and the live-derivation word (no schedule, no polling — the stale
	// value never renders as current); the AG-4 blocked dispatch of the
	// world keeps its honest words.
	t.Run("row-7 stale status: the freshness without the schedule", func(t *testing.T) {
		caseID, _, operation := statusApprovedOrder(t, w, svc, adapter, operator, backup, "s7", fmtOrderTarget(937))
		projection := statusView(t, svc, operator, w, caseID)
		if projection.Freshness.LastConfirmedAt.IsZero() {
			t.Fatal("the freshness time of the moved material is zero")
		}
		if !strings.Contains(projection.Freshness.Word, "живая деривация при открытии") {
			t.Fatalf("the freshness word: %q", projection.Freshness.Word)
		}
		if !strings.Contains(projection.Freshness.Word, "не подключены") {
			t.Fatalf("the AG-4 word of the freshness: %q", projection.Freshness.Word)
		}
		// The 1C projection stays unknown with the OQ-4 pin (the
		// ABCP→1C delay row of the matrix).
		if !strings.Contains(projection.Systems[1].Word, "OQ-4") {
			t.Fatalf("the 1C word without the OQ-4 pin: %q", projection.Systems[1].Word)
		}
		// The notification word (OQ-1/OQ-2) and the dispute word (AC-14)
		// ride the projection.
		if !strings.Contains(projection.NotificationWord, "не отправляются") {
			t.Fatalf("the notification word: %q", projection.NotificationWord)
		}
		if !strings.Contains(projection.DisputeWord, "не переписывает историю") {
			t.Fatalf("the dispute word: %q", projection.DisputeWord)
		}
		// The supplier system block speaks «не записано» (AG-4).
		if projection.Systems[2].Word != actionDomain.OrderStatusSupplierNotRecordedWord {
			t.Fatalf("the supplier word without facts: %q", projection.Systems[2].Word)
		}
		_ = operation
	})

	// Row 8 — the MISSING ANCHOR and the FOREIGN RLS: the case without
	// an order operation speaks the honest miss word; the foreign tenant
	// sees NOTHING (the store RLS mirror); NO supplier-send column
	// exists on the fact table (the OQ-3 structural probe) and the
	// UPDATE/DELETE paths are absent (the append-only backstop).
	t.Run("row-8 missing anchor, RLS blindness and supplier-send probe zero", func(t *testing.T) {
		// A case of the world WITHOUT an order operation: the projection
		// reads the honest miss word (never an error).
		caseID, _ := orderRowCase(t, w, "s8")
		projection, err := svc.ReadOrderStatusView(ctx, operator, w.tenant, caseID, orderNow.Add(50*time.Minute))
		if err != nil {
			t.Fatalf("the projection of the case without an order refused: %v", err)
		}
		if projection.Stages[0].Confirmed || !strings.Contains(projection.Stages[0].Word, "операции заказа в кейсе нет") {
			t.Fatalf("the honest miss word: %+v", projection.Stages[0])
		}
		if projection.CurrentStage != "" {
			t.Fatalf("the current stage without an order: %q", projection.CurrentStage)
		}
		// The RLS blindness: the foreign tenant context sees NO facts
		// (the store RLS mirror).
		foreign := actionStore.NewPostgres(runtimePool)
		otherTenant := id.MustParse("01994f64-6c00-7000-8000-00000000f030")
		otherScope, err := tenancy.New(id.MustParse("01994f64-6c00-7000-8000-00000000f031"), otherTenant)
		if err != nil {
			t.Fatal(err)
		}
		facts, err := foreign.ReadCaseSupplierFacts(ctx, otherScope, caseID)
		if err != nil || len(facts) != 0 {
			t.Fatalf("the foreign tenant sees NO facts, got %d err=%v", len(facts), err)
		}
		// The SQL structural probe: NO send/supplier-action column exists
		// on the fact table (the OQ-3 fence — the kind words are the
		// recorded claims, never the commands).
		rows, err := adminPool.Query(ctx,
			`select column_name from information_schema.columns
                         where table_schema='action' and table_name='order_supplier_facts'
                         and (column_name ilike '%send%' or column_name ilike '%dispatch%' or column_name ilike '%command%')`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if rows.Next() {
			t.Fatal("the fact table carries a supplier-send column")
		}
		// The append-only backstop: the UPDATE and DELETE paths are
		// absent for the runtime role (the grants are INSERT/SELECT only).
		if _, err := runtimePool.Exec(ctx, "update action.order_supplier_facts set supplier_order_id='X' where false"); err == nil {
			t.Fatal("the UPDATE path of the fact table is open")
		}
		if _, err := runtimePool.Exec(ctx, "delete from action.order_supplier_facts where false"); err == nil {
			t.Fatal("the DELETE path of the fact table is open")
		}
	})
}
