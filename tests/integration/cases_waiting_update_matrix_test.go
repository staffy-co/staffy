//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	identityDomain "github.com/artemiosu/staffy/internal/identity/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
)

// TestWaitingUpdateMatrixRowsBySpec is the Story 2.16 step-04 matrix: the
// honest rows of the truthful waiting surface walked by name over the real
// app contour (the step-03 test pinned the command skeleton; this matrix
// walks the per-state derivations and the honest boundaries). The seed is
// 985 (the free slot of the 960..999 range — the worlds are unique, the
// membership collision lesson); the matrix id sequence takes 996000 (the
// range is unique: 991000 continuation, 993000 accommodation, 994000
// degradation, 995000 handoff); the member suffixes live in the free 985
// slot space (no world derives from the seed 985).
func TestWaitingUpdateMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	w := newCasesWorld(t, 985)
	// The matrix world carries its own wide id sequence (the 2.10..2.13
	// precedent: the many-commanded rows drink the generator dry).
	matrixIDs := &sequenceIDs{values: make([]id.UUID, 512)}
	for i := range matrixIDs.values {
		matrixIDs.values[i] = id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 996000+i))
	}
	w.cases = w.buildCasesServiceClock(matrixIDs, fixedClock{ownershipNow})
	ctx := context.Background()
	now := ownershipNow

	// The main case of the distinction rows: the ask, the promise walk
	// through requested → assigned → acknowledged, then the return.
	caseA, _ := w.acceptedCase(t, "evt-waiting-matrix-1", "sess-waiting-matrix-1", `{"call":"inbound-call","from":"+79000000981"}`)
	turnA := w.modeTurnEvent(t, caseA, "sess-waiting-matrix-1", "evt-waiting-matrix-turn-1", `{"say":"позовите человека"}`)
	// The missed family case: the short ack deadline and the 15m promise.
	caseB, _ := w.acceptedCase(t, "evt-waiting-matrix-2", "sess-waiting-matrix-2", `{"call":"inbound-call","from":"+79000000982"}`)
	turnB := w.modeTurnEvent(t, caseB, "sess-waiting-matrix-2", "evt-waiting-matrix-turn-2", `{"say":"позовите человека"}`)
	// The refusal case of the declined round.
	caseC, _ := w.acceptedCase(t, "evt-waiting-matrix-3", "sess-waiting-matrix-3", `{"call":"inbound-call","from":"+79000000983"}`)
	turnC := w.modeTurnEvent(t, caseC, "sess-waiting-matrix-3", "evt-waiting-matrix-turn-3", `{"say":"позовите человека"}`)

	promise := func(actor identityDomain.Principal, caseNo id.UUID, word, key string, at time.Time) (casesAPI.WorkOutcome, error) {
		return w.cases.PromiseUpdate(ctx, casesAPI.PromiseUpdateCommand{
			Actor: actor, TenantID: w.tenant, CaseID: caseNo,
			DeadlineWord: word, IdempotencyKey: key, Now: at,
		})
	}
	viewOf := func(caseNo id.UUID) casesAPI.WaitingUpdateView {
		view, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseNo)
		if err != nil {
			t.Fatal(err)
		}
		return view.Waiting
	}
	viewOfBy := func(actor identityDomain.Principal, caseNo id.UUID) casesAPI.WaitingUpdateView {
		view, err := w.cases.CaseView(ctx, actor, w.tenant, caseNo)
		if err != nil {
			t.Fatal(err)
		}
		return view.Waiting
	}

	// The ask of the main case (the requested round of the distinction
	// rows).
	if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseA,
		TurnRef: turnA, ExpectedModeVersion: 1,
		TargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, AckDeadline: casesDomain.HandoffAck4h,
		IdempotencyKey: "waiting-matrix-request-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	// Row 1 — the fresh ask with the 15m promise: the FIRST distinction
	// of AC-1 («ищем специалиста» — the human is not connected yet), the
	// server truth of the promised time (now + 15m, never a request
	// field), the promise number 1 of the monotonic per-case sequence.
	t.Run("row-1 requested round: searching status, 15m promise, server time", func(t *testing.T) {
		if _, err := promise(w.ownershipActor(), caseA, casesDomain.UpdateDeadline15m, "waiting-matrix-promise-1", now); err != nil {
			t.Fatalf("the promise over the fresh ask must land, got %v", err)
		}
		var promisedAt *time.Time
		var word string
		var promiseNo int64
		if err := adminPool.QueryRow(ctx, `
                        select promise_no, deadline_word, promised_at
                        from cases.handoff_update_promises
                        where tenant_id=$1 and case_id=$2 and promise_no=1`,
			w.tenant, caseA).Scan(&promiseNo, &word, &promisedAt); err != nil {
			t.Fatal(err)
		}
		if promiseNo != 1 || word != casesDomain.UpdateDeadline15m {
			t.Fatalf("the first promise of the case carries the word and the number, got no=%d word=%s", promiseNo, word)
		}
		if promisedAt == nil || !promisedAt.Equal(now.Add(15*time.Minute)) {
			t.Fatalf("the 15m word computes now+15m by the SERVER, got %v", promisedAt)
		}
		view := viewOf(caseA)
		if !view.HasPromise || view.ClientStatus != casesDomain.ClientWaitingSearching ||
			view.ClientStatusText != casesDomain.ClientWaitingStatusText(casesDomain.ClientWaitingSearching) {
			t.Fatalf("the requested round derives the FIRST distinction (searching), got %+v", view)
		}
		if view.PromiseNo != 1 || view.DeadlineWord != casesDomain.UpdateDeadline15m ||
			view.State != casesDomain.UpdatePromisePending {
			t.Fatalf("the promise №1 derives pending over the fresh ask, got %+v", view)
		}
	})

	// Row 2 — the honest unknown word over the same round: the NULL
	// promised time (the material CHECK pair — a fabricated timestamp
	// NEVER accompanies «срок неизвестен»), the honest no-deadline state,
	// the LAST promise speaks.
	t.Run("row-2 unknown word: NULL time, honest no-deadline state", func(t *testing.T) {
		if _, err := promise(w.ownershipActor(), caseA, casesDomain.UpdateDeadlineUnknown, "waiting-matrix-promise-2", now); err != nil {
			t.Fatalf("the honest unknown promise must land, got %v", err)
		}
		var unknownAt *time.Time
		if err := adminPool.QueryRow(ctx, `
                        select promised_at from cases.handoff_update_promises
                        where tenant_id=$1 and case_id=$2 and promise_no=2`,
			w.tenant, caseA).Scan(&unknownAt); err != nil {
			t.Fatal(err)
		}
		if unknownAt != nil {
			t.Fatalf("the unknown word NEVER carries a timestamp (the CHECK pair), got %v", unknownAt)
		}
		view := viewOf(caseA)
		if view.PromiseNo != 2 || view.DeadlineWord != casesDomain.UpdateDeadlineUnknown ||
			!view.PromisedAt.IsZero() || view.State != casesDomain.UpdatePromiseUnknown {
			t.Fatalf("the LAST promise speaks the honest unknown state, got %+v", view)
		}
		if view.DeadlineText != casesDomain.UpdateDeadlineText(casesDomain.UpdateDeadlineUnknown) {
			t.Fatalf("the deadline text is the honest unknown text, got %q", view.DeadlineText)
		}
	})

	// Row 3 — the assignment flips the status: the SECOND distinction of
	// AC-1 («назначен ≠ принял» — two honest words, never one); the
	// promise line keeps its own truth (the last promise stays unknown).
	t.Run("row-3 assignment: the second distinction named-not-accepted", func(t *testing.T) {
		if _, err := w.cases.AssignHumanHandoff(ctx, casesAPI.AssignHumanHandoffCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseA,
			Assignee: w.backup, Backup: w.owner.ID, AckDeadline: casesDomain.HandoffAck4h,
			IdempotencyKey: "waiting-matrix-assign-1", Now: now,
		}); err != nil {
			t.Fatal(err)
		}
		view := viewOf(caseA)
		if view.ClientStatus != casesDomain.ClientWaitingAssigned ||
			view.ClientStatusText != casesDomain.ClientWaitingStatusText(casesDomain.ClientWaitingAssigned) {
			t.Fatalf("the assigned round derives the SECOND distinction, got %+v", view)
		}
		if view.PromiseNo != 2 || view.DeadlineWord != casesDomain.UpdateDeadlineUnknown ||
			view.State != casesDomain.UpdatePromiseUnknown {
			t.Fatalf("the promise line keeps its own truth under the assignment, got %+v", view)
		}
	})

	// Row 4 — the assignee sees the promise line BEFORE the
	// acknowledgement: the waiting view is the shared material of the
	// round (the promise never waits for the acceptance).
	t.Run("row-4 assignee sees the promise line before ack", func(t *testing.T) {
		view := viewOfBy(w.backupActor(), caseA)
		if !view.HasPromise || view.PromiseNo != 2 || view.DeadlineWord != casesDomain.UpdateDeadlineUnknown {
			t.Fatalf("the assignee reads the promise line BEFORE the ack, got %+v", view)
		}
		if view.ClientStatus != casesDomain.ClientWaitingAssigned {
			t.Fatalf("the assignee reads the honest second distinction too, got %+v", view)
		}
	})

	// Row 5 — the acknowledgement: the THIRD distinction of AC-1
	// («специалист принял обращение»); the promise of the accepted round
	// keeps ticking honestly (the known word of the accepted case — the
	// updates are still promised while the human works).
	t.Run("row-5 acknowledged: the third distinction, promise keeps ticking", func(t *testing.T) {
		if _, err := w.cases.AcknowledgeHumanHandoff(ctx, casesAPI.AcknowledgeHumanHandoffCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseA,
			ExpectedEpoch: 1, IdempotencyKey: "waiting-matrix-ack-1", Now: now,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := promise(w.ownershipActor(), caseA, casesDomain.UpdateDeadline1h, "waiting-matrix-promise-3", now); err != nil {
			t.Fatalf("the promise of the accepted round must land (the updates are still owed), got %v", err)
		}
		view := viewOf(caseA)
		if view.ClientStatus != casesDomain.ClientWaitingAccepted ||
			view.ClientStatusText != casesDomain.ClientWaitingStatusText(casesDomain.ClientWaitingAccepted) {
			t.Fatalf("the acknowledged round derives the THIRD distinction, got %+v", view)
		}
		if view.PromiseNo != 3 || view.DeadlineWord != casesDomain.UpdateDeadline1h ||
			view.State != casesDomain.UpdatePromisePending {
			t.Fatalf("the known promise of the accepted round keeps ticking, got %+v", view)
		}
	})

	// The missed family of caseB: the ask, the assignment with the SHORT
	// ack deadline and the 15m promise, then the world-clock shift.
	if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseB,
		TurnRef: turnB, ExpectedModeVersion: 1,
		TargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, AckDeadline: casesDomain.HandoffAck15m,
		IdempotencyKey: "waiting-matrix-request-2", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.AssignHumanHandoff(ctx, casesAPI.AssignHumanHandoffCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseB,
		Assignee: w.backup, Backup: w.owner.ID, AckDeadline: casesDomain.HandoffAck15m,
		IdempotencyKey: "waiting-matrix-assign-2", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := promise(w.ownershipActor(), caseB, casesDomain.UpdateDeadline15m, "waiting-matrix-promise-4", now); err != nil {
		t.Fatal(err)
	}
	// The row bytes of the promise BEFORE the shift (the mutation probe
	// compares them after the missed derivation).
	var rowWord, rowActor string
	var rowNo int64
	var rowPromisedAt, rowObservedAt, rowReceivedAt, rowCreatedAt time.Time
	if err := adminPool.QueryRow(ctx, `
                select promise_no, deadline_word, promised_at, observed_at, received_at, created_at, actor_id::text
                from cases.handoff_update_promises
                where tenant_id=$1 and case_id=$2 and promise_no=1`,
		w.tenant, caseB).Scan(&rowNo, &rowWord, &rowPromisedAt, &rowObservedAt, &rowReceivedAt, &rowCreatedAt, &rowActor); err != nil {
		t.Fatal(err)
	}
	// The digest of the facts BEFORE the promise walk of caseA (the
	// perimeter row compares it after every promise write).
	digestBefore := func() string {
		view, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseA)
		if err != nil {
			t.Fatal(err)
		}
		return view.Policy.FactsDigest
	}()
	// The promise count of the tenant BEFORE the shift (the read-time
	// rows never grow by derivations).
	var countBefore int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.handoff_update_promises where tenant_id=$1`,
		w.tenant).Scan(&countBefore); err != nil {
		t.Fatal(err)
	}

	// The world-clock shift: the later moment of the derivations (the
	// 2.13 row-21 precedent — the service rebuilds over the fixed later
	// clock; the rows and the commands stay untouched).
	laterNow := ownershipNow.Add(2 * time.Hour)
	w.cases = w.buildCasesServiceClock(matrixIDs, fixedClock{laterNow})

	// Row 6 — the missed promise of the overdue round: the read-time
	// derivations of the later moment — the acceptance deadline AND the
	// promised time both passed; the round derives the honest overdue
	// word of 2.13, the promise the honest missed word with the next
	// step; the unknown word of another round never misses (the honest
	// separation of the two deadlines: ack_deadline is the acceptance
	// promise, promised_at is the update promise — different owners).
	t.Run("row-6 missed read-time: the overdue round, the honest missed word", func(t *testing.T) {
		view := viewOf(caseB)
		if view.ClientStatus != casesDomain.ClientWaitingOverdue ||
			view.ClientStatusText != casesDomain.ClientWaitingStatusText(casesDomain.ClientWaitingOverdue) {
			t.Fatalf("the un-acknowledged round derives the honest overdue acceptance, got %+v", view)
		}
		if !view.HasPromise || view.PromiseNo != 1 || view.DeadlineWord != casesDomain.UpdateDeadline15m ||
			view.State != casesDomain.UpdatePromiseMissed {
			t.Fatalf("the passed promised time derives the honest missed word, got %+v", view)
		}
		if view.StateText != casesDomain.UpdatePromiseStateText(casesDomain.UpdatePromiseMissed) ||
			view.MissedNextStepText != casesDomain.UpdatePromiseMissedNextStepText {
			t.Fatalf("the missed state carries the honest texts, got %+v", view)
		}
		if !strings.Contains(view.PromiseLineText, casesDomain.UpdatePromiseStateText(casesDomain.UpdatePromiseMissed)) ||
			!strings.Contains(view.PromiseLineText, casesDomain.UpdatePromiseMissedNextStepText) {
			t.Fatalf("the client line composes the missed state with the next step, got %q", view.PromiseLineText)
		}
		// The honest separation of the two deadlines: the ack
		// deadline is the acceptance promise of 2.13, the promised
		// time is the update promise — different owners. caseA: the
		// accepted round keeps the THIRD distinction while its own
		// 1h promise derives missed at the later moment (the
		// missed update NEVER touches the round — AC-3).
		acceptedView := viewOf(caseA)
		if acceptedView.ClientStatus != casesDomain.ClientWaitingAccepted {
			t.Fatalf("the accepted round keeps the third distinction at the later moment, got %+v", acceptedView)
		}
		if acceptedView.DeadlineWord != casesDomain.UpdateDeadline1h ||
			acceptedView.State != casesDomain.UpdatePromiseMissed {
			t.Fatalf("the accepted round derives its OWN missed promise honestly, got %+v", acceptedView)
		}
	})

	// Row 7 — the missed derivation NEVER mutates: the append-only row
	// keeps its bytes, the row count never grows by a derivation, the
	// case stays open and the round stays open (the promise NEVER
	// resolves the round — AC-3: the lifecycle transitions belong to the
	// 2.13 commands only).
	t.Run("row-7 missed never mutates: row bytes, count, open round", func(t *testing.T) {
		var rowWord2, rowActor2 string
		var rowNo2 int64
		var rowPromisedAt2, rowObservedAt2, rowReceivedAt2, rowCreatedAt2 time.Time
		if err := adminPool.QueryRow(ctx, `
                        select promise_no, deadline_word, promised_at, observed_at, received_at, created_at, actor_id::text
                        from cases.handoff_update_promises
                        where tenant_id=$1 and case_id=$2 and promise_no=1`,
			w.tenant, caseB).Scan(&rowNo2, &rowWord2, &rowPromisedAt2, &rowObservedAt2, &rowReceivedAt2, &rowCreatedAt2, &rowActor2); err != nil {
			t.Fatal(err)
		}
		if rowNo2 != rowNo || rowWord2 != rowWord || rowActor2 != rowActor ||
			!rowPromisedAt2.Equal(rowPromisedAt) || !rowObservedAt2.Equal(rowObservedAt) ||
			!rowReceivedAt2.Equal(rowReceivedAt) || !rowCreatedAt2.Equal(rowCreatedAt) {
			t.Fatalf("the missed derivation keeps the row bytes, got %v vs %v", rowPromisedAt2, rowPromisedAt)
		}
		var countAfter int
		if err := adminPool.QueryRow(ctx, `
                        select count(*) from cases.handoff_update_promises where tenant_id=$1`,
			w.tenant).Scan(&countAfter); err != nil {
			t.Fatal(err)
		}
		if countAfter != countBefore {
			t.Fatalf("a derivation never writes rows, got %d want %d", countAfter, countBefore)
		}
		// The case and the round stay open (AC-3: the promise never
		// resolves the round; the missed word is the read-time truth of
		// the moment).
		view, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseB)
		if err != nil {
			t.Fatal(err)
		}
		if view.Case.State != casesDomain.CaseStateOpen {
			t.Fatalf("the case stays open under the missed promise, got %s", view.Case.State)
		}
		if len(view.Handoff.Requests) != 1 || !view.Handoff.Requests[0].Active ||
			view.Handoff.Requests[0].State != casesDomain.HandoffStateOverdue {
			t.Fatalf("the round stays OPEN and derives overdue (never resolved by the promise), got %+v", view.Handoff.Requests)
		}
	})

	// Row 8 — the promise over the OVERDUE round lands honestly: the
	// overdue acceptance does not refuse the update promise (the update
	// is still owed to the client by whoever continues the case — the
	// anchor set accepts the open round of the honest lateness).
	t.Run("row-8 promise over the overdue round accepted", func(t *testing.T) {
		if _, err := promise(w.ownershipActor(), caseB, casesDomain.UpdateDeadline1h, "waiting-matrix-promise-5", laterNow); err != nil {
			t.Fatalf("the overdue round accepts the update promise, got %v", err)
		}
		var promisedAt *time.Time
		if err := adminPool.QueryRow(ctx, `
                        select promised_at from cases.handoff_update_promises
                        where tenant_id=$1 and case_id=$2 and promise_no=2`,
			w.tenant, caseB).Scan(&promisedAt); err != nil {
			t.Fatal(err)
		}
		if promisedAt == nil || !promisedAt.Equal(laterNow.Add(time.Hour)) {
			t.Fatalf("the new promise computes later-now+1h by the server, got %v", promisedAt)
		}
	})

	// Row 9 — the missed promise heals by the NEW promise only: №2
	// derives pending (the missed word is the truth of the moment, never
	// a stored status), №1 derives superseded read-time — the append-only
	// rows never mutate (the «последняя говорит» discipline of the 2.15
	// precedent).
	t.Run("row-9 the new promise heals: supersede derivation-only", func(t *testing.T) {
		view := viewOf(caseB)
		if view.PromiseNo != 2 || view.DeadlineWord != casesDomain.UpdateDeadline1h ||
			view.State != casesDomain.UpdatePromisePending {
			t.Fatalf("the LAST promise speaks pending — the missed word healed by the new promise, got %+v", view)
		}
		if view.MissedNextStepText != "" {
			t.Fatalf("the healed view carries NO missed next step, got %q", view.MissedNextStepText)
		}
		var rowCount int
		if err := adminPool.QueryRow(ctx, `
                        select count(*) from cases.handoff_update_promises where tenant_id=$1 and case_id=$2`,
			w.tenant, caseB).Scan(&rowCount); err != nil {
			t.Fatal(err)
		}
		if rowCount != 2 {
			t.Fatalf("both promise rows live append-only, got %d", rowCount)
		}
		var rowWord2 string
		var rowPromisedAt2 time.Time
		if err := adminPool.QueryRow(ctx, `
                        select deadline_word, promised_at from cases.handoff_update_promises
                        where tenant_id=$1 and case_id=$2 and promise_no=1`,
			w.tenant, caseB).Scan(&rowWord2, &rowPromisedAt2); err != nil {
			t.Fatal(err)
		}
		if rowWord2 != rowWord || !rowPromisedAt2.Equal(rowPromisedAt) {
			t.Fatalf("the superseded row keeps its bytes (derivation-only), got word=%s at=%v", rowWord2, rowPromisedAt2)
		}
	})

	// Row 10 — the declined round refuses: the assignment round is
	// consumed by the refusal (the re-assignment decision belongs to the
	// human) — the promise names nobody over the consumed round.
	t.Run("row-10 declined round refuses with the mirror words", func(t *testing.T) {
		if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseC,
			TurnRef: turnC, ExpectedModeVersion: 1,
			TargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, AckDeadline: casesDomain.HandoffAck1h,
			IdempotencyKey: "waiting-matrix-request-3", Now: laterNow,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.cases.AssignHumanHandoff(ctx, casesAPI.AssignHumanHandoffCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseC,
			Assignee: w.backup, Backup: w.owner.ID, AckDeadline: casesDomain.HandoffAck1h,
			IdempotencyKey: "waiting-matrix-assign-3", Now: laterNow,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.cases.DeclineHumanHandoff(ctx, casesAPI.DeclineHumanHandoffCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseC,
			IdempotencyKey: "waiting-matrix-decline-1", Now: laterNow,
		}); err != nil {
			t.Fatal(err)
		}
		_, err := promise(w.ownershipActor(), caseC, casesDomain.UpdateDeadline1h, "waiting-matrix-declined-1", laterNow)
		assertRuleError(t, err, "не принимает обещаний")
		// The declined round stays the honest SEARCHING status of the
		// client (the refusal keeps the search alive — no promise
		// material was written).
		view := viewOf(caseC)
		if view.HasPromise {
			t.Fatalf("the refused promise never lands, got %+v", view)
		}
	})

	// Row 11 — the returned round ends the waiting honestly: the promise
	// refuses with the mirror words, the closed promises stay the
	// readable history (the last promise derives closed with the honest
	// ending word, the client status the honest returned word).
	t.Run("row-11 returned round: refusal, closed history readable", func(t *testing.T) {
		if _, err := w.cases.ReturnToStaffy(ctx, casesAPI.ReturnToStaffyCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseA,
			Result: casesDomain.HandoffResultIssueResolved, ExpectedEpoch: 2,
			IdempotencyKey: "waiting-matrix-return-1", Now: laterNow,
		}); err != nil {
			t.Fatal(err)
		}
		_, err := promise(w.ownershipActor(), caseA, casesDomain.UpdateDeadline1h, "waiting-matrix-returned-1", laterNow)
		assertRuleError(t, err, "не принимает обещаний")
		view := viewOf(caseA)
		if view.ClientStatus != casesDomain.ClientWaitingReturned || view.State != casesDomain.UpdatePromiseClosed {
			t.Fatalf("the returned round ends the waiting and closes the promise honestly, got %+v", view)
		}
		if view.StateText != casesDomain.UpdatePromiseStateText(casesDomain.UpdatePromiseClosed) {
			t.Fatalf("the closed state carries the honest ending text, got %q", view.StateText)
		}
	})

	// Row 12 — the foreign tenant reads the plain 404 BEFORE the
	// material guards (the review-scoped member of ANOTHER tenant never
	// promises here; the invalid word never masks the refusal — the
	// honest 404 of the scope resolution, the 2.14 row-18 precedent).
	t.Run("row-12 foreign tenant 404 before the material guards", func(t *testing.T) {
		foreignTenant := createForeignTenant(t, w)
		foreign := inviteMember(t, w.identity, w.owner, foreignTenant, "waiting-matrix-foreign@example.org", "expert_qa",
			[]identityDomain.Scope{identityDomain.Review}, "000000009853", "000000009854")
		_, err := promise(foreign, caseB, "update_2h", "waiting-matrix-foreign-1", laterNow)
		if !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the member of another tenant reads the plain 404, got %v", err)
		}
		// The read stays the same honest 404.
		if _, err := w.cases.CaseView(ctx, foreign, w.tenant, caseB); !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the foreign member never reads the case view, got %v", err)
		}
	})

	// Row 13 — the non-member reads the plain 404 too (a principal
	// without ANY membership of the tenant — the scope resolution
	// refuses before the material guards; the 2.14 row-18 precedent).
	t.Run("row-13 non-member 404", func(t *testing.T) {
		nonMember := actorPrincipal{principalID: nextWaitingUpdateProbeID(), stepUp: true}.principal()
		_, err := promise(nonMember, caseB, casesDomain.UpdateDeadline1h, "waiting-matrix-nonmember-1", laterNow)
		if !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the non-member reads the plain 404, got %v", err)
		}
	})

	// Row 14 — the anti-disclosure probe of AC-1: the client-facing
	// texts carry the dictionary words ONLY — the operator identifiers,
	// the tenant identifier, the epochs and the UUID material NEVER
	// enter (byte-exact absence over every text of the waiting view).
	t.Run("row-14 anti-disclosure: no persona, no epochs, no ids", func(t *testing.T) {
		view := viewOf(caseB)
		texts := []string{
			view.ClientStatusText, view.DeadlineText, view.StateText,
			view.PromiseLineText, view.MissedNextStepText,
		}
		uuidPattern := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}`)
		for _, text := range texts {
			if strings.Contains(text, w.ownershipActor().ID.String()) ||
				strings.Contains(text, w.backupActor().ID.String()) ||
				strings.Contains(text, w.tenant.String()) ||
				strings.Contains(text, "эпох") || strings.Contains(text, "epoch") {
				t.Fatalf("the persona material never enters the client texts, got %q", text)
			}
			if uuidPattern.MatchString(text) {
				t.Fatalf("the UUID material never enters the client texts, got %q", text)
			}
		}
	})

	// Row 15 — the dictionary discipline of the words: every word of the
	// view belongs to the fixed compile-time dictionaries (the client
	// status, the promise state, the deadline word — never an invented
	// word of a runtime configuration).
	t.Run("row-15 dictionary words only", func(t *testing.T) {
		view := viewOf(caseB)
		if !casesDomain.ClientWaitingStatusInDictionary(view.ClientStatus) {
			t.Fatalf("the client status word belongs to the dictionary, got %q", view.ClientStatus)
		}
		if !casesDomain.UpdateDeadlineInDictionary(view.DeadlineWord) {
			t.Fatalf("the deadline word belongs to the dictionary, got %q", view.DeadlineWord)
		}
		if !inStrings(casesDomain.UpdatePromiseStates, view.State) {
			t.Fatalf("the promise state word belongs to the dictionary, got %q", view.State)
		}
	})

	// Row 16 — the control branch: a case without the lifecycle renders
	// the ZERO waiting view (no round, no block — the page renders
	// byte-for-byte unchanged; the promise never floats without the ask).
	t.Run("row-16 control branch: no lifecycle, zero view", func(t *testing.T) {
		fresh, _ := w.acceptedCase(t, "evt-waiting-matrix-4", "sess-waiting-matrix-4", `{"call":"inbound-call","from":"+79000000984"}`)
		view := viewOf(fresh)
		if view.HasPromise || view.ClientStatus != "" || view.State != "" ||
			view.PromiseLineText != "" || view.MissedNextStepText != "" {
			t.Fatalf("the case without the lifecycle renders the ZERO waiting view, got %+v", view)
		}
		if len(view.DeadlineWords) != 4 {
			t.Fatalf("the form dictionary renders regardless of the material, got %v", view.DeadlineWords)
		}
	})

	// Row 17 — the fact perimeter: the promise writes never enter the
	// facts digest of the case (the promises are the operational rows,
	// never the facts — the 2.13 row-22 precedent: the digest never
	// moves by a lifecycle or promise write).
	t.Run("row-17 facts digest never moves", func(t *testing.T) {
		if digestBefore == "" {
			t.Fatalf("the digest of the promise walk must be a live value")
		}
		digestAfter := func() string {
			view, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseA)
			if err != nil {
				t.Fatal(err)
			}
			return view.Policy.FactsDigest
		}()
		if digestAfter != digestBefore {
			t.Fatalf("the promise writes never move the facts digest, got %q want %q", digestAfter, digestBefore)
		}
	})
}

// inStrings reports whether the word belongs to the fixed list (the
// dictionary probe of the matrix rows).
func inStrings(list []string, word string) bool {
	for _, candidate := range list {
		if candidate == word {
			return true
		}
	}
	return false
}
