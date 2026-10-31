//go:build integration

package integration_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/jackc/pgx/v5/pgconn"
)

// TestWaitingUpdateAppCommandAndView pins the Story 2.16 step-03 app
// contour: the operator promise command (the scope gate BEFORE the
// material guards, the dictionary guard, the replay and the key
// conflict, the round guards of the store mirror words, the terminal
// boundary) and the truthful waiting projection of the case view (the
// client waiting status of the last round in the three distinctions of
// AC-1, the last promise with the SERVER-computed promised time, the
// unknown word without a fabricated timestamp, the supersede
// derivation-only, the control branch without the promise material).
// The full integration matrix of the slice (the missed read-time
// branch, the per-state derivations) extends in step-04.
func TestWaitingUpdateAppCommandAndView(t *testing.T) {
	w := newCasesWorld(t, 975)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-waiting-app-1", "sess-waiting-app-1", `{"call":"inbound-call","from":"+79000000701"}`)
	now := ownershipNow

	// The open assigned round of the promise anchor: the ask, the
	// assignment (the promise is the duty of communication — the
	// NON-owner operator may promise, the precedent of the 2.15
	// correction).
	turn := w.modeTurnEvent(t, caseID, "sess-waiting-app-1", "evt-waiting-app-turn-1", `{"say":"позовите человека"}`)
	if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		TurnRef: turn, ExpectedModeVersion: 1,
		TargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, AckDeadline: casesDomain.HandoffAck1h,
		IdempotencyKey: "waiting-app-request-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.AssignHumanHandoff(ctx, casesAPI.AssignHumanHandoffCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		Assignee: w.backup, Backup: w.owner.ID, AckDeadline: casesDomain.HandoffAck4h,
		IdempotencyKey: "waiting-app-assign-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	promise := func(actor identityDomain.Principal, word, key string) (casesAPI.WorkOutcome, error) {
		return w.cases.PromiseUpdate(ctx, casesAPI.PromiseUpdateCommand{
			Actor: actor, TenantID: w.tenant, CaseID: caseID,
			DeadlineWord: word, IdempotencyKey: key, Now: now,
		})
	}
	viewOf := func() casesAPI.WaitingUpdateView {
		view, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		return view.Waiting
	}

	// The scope gate BEFORE the material guards (the M1 precedent): the
	// audit-only member is forbidden with the scope words — the invalid
	// word never masks the refusal. The member suffixes live in the free
	// 974 slot space (no world derives from the seed 974 — the world
	// suffix ranges stay collision-free, the 2.14/2.16 seed lesson).
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "waiting-app-audit@example.org", "expert_qa",
		[]identityDomain.Scope{identityDomain.Audit}, "000000009741", "000000009742")
	auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
	if _, err := promise(auditOnly, "update_2h", "waiting-app-audit-1"); !errors.Is(err, identityDomain.ErrForbidden) ||
		!strings.Contains(err.Error(), "review или configuration") {
		t.Fatalf("the audit-only member is forbidden BEFORE the material guards, got %v", err)
	}

	// The domain guard of the ONLY request material (the honest 400
	// before SQL): the word of the fixed dictionary.
	_, err := promise(w.ownershipActor(), "update_2h", "waiting-app-word-1")
	assertRuleError(t, err, "не входит в словарь")

	// The fresh case without the ask: the promise never floats without
	// the round (the store mirror words of the absent guard).
	freshCase, _ := w.acceptedCase(t, "evt-waiting-app-2", "sess-waiting-app-2", `{"call":"inbound-call","from":"+79000000702"}`)
	_, err = w.cases.PromiseUpdate(ctx, casesAPI.PromiseUpdateCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: freshCase,
		DeadlineWord: casesDomain.UpdateDeadline1h, IdempotencyKey: "waiting-app-absent-1", Now: now,
	})
	assertRuleError(t, err, "Запроса человека не существует")

	// The control branch of the slice BEFORE the first promise: the
	// round WITHOUT promises renders the ZERO waiting view (the block
	// appears only with the promise material — the 2.13 panel stays the
	// honest surface of the round without promises; the form dictionary
	// renders regardless of the material).
	view := viewOf()
	if view.HasPromise || view.ClientStatus != "" ||
		view.PromiseLineText != "" || view.State != "" {
		t.Fatalf("the round without promises renders the ZERO waiting view (the control branch), got %+v", view)
	}
	if len(view.DeadlineWords) != 4 {
		t.Fatalf("the form dictionary renders regardless of the material, got %v", view.DeadlineWords)
	}

	// The honest promise of the known word by the NON-owner operator: the
	// outcome carries the promise id with the deadline word ONLY (the
	// promise number and the computed time are the SERVER truths the
	// reader learns from the waiting view).
	first, err := promise(w.backupActor(), casesDomain.UpdateDeadline1h, "waiting-app-promise-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Command != "promise_handoff_update" || first.PromiseDeadlineWord != casesDomain.UpdateDeadline1h || !first.PromiseID.Valid() {
		t.Fatalf("the promise outcome carries the id with the deadline word, got %+v", first)
	}

	// The server truths of the row: the promise number 1 (the per-case
	// monotonic sequence under the locks), the promised time computed by
	// the SERVER (now + window of the word — never a request field), the
	// responsible actor of the command.
	var promiseNo, requestNo int64
	var promisedAt *time.Time
	var actorID id.UUID
	var word string
	if err := adminPool.QueryRow(ctx, `
                select promise_no, request_no, deadline_word, promised_at, actor_id
                from cases.handoff_update_promises
                where tenant_id=$1 and case_id=$2 and promise_no=1`,
		w.tenant, caseID).Scan(&promiseNo, &requestNo, &word, &promisedAt, &actorID); err != nil {
		t.Fatal(err)
	}
	if promiseNo != 1 || requestNo != 1 || word != casesDomain.UpdateDeadline1h || actorID != w.backupActor().ID {
		t.Fatalf("the server truths of the promise row, got no=%d request=%d word=%s actor=%s", promiseNo, requestNo, word, actorID)
	}
	if promisedAt == nil || !promisedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("the promised time is the SERVER truth now+window, got %v", promisedAt)
	}

	// The waiting view of the case page: the client waiting status of the
	// assigned round (the second distinction of AC-1 — «назначен ≠
	// принял»), the last promise with the deadline word, the state
	// pending, the honest client line composition and the form
	// dictionary.
	view = viewOf()
	if !view.HasPromise || view.ClientStatus != casesDomain.ClientWaitingAssigned ||
		view.ClientStatusText != casesDomain.ClientWaitingStatusText(casesDomain.ClientWaitingAssigned) {
		t.Fatalf("the waiting status carries the second distinction of AC-1, got %+v", view)
	}
	if view.PromiseNo != 1 || view.DeadlineWord != casesDomain.UpdateDeadline1h ||
		view.DeadlineText != casesDomain.UpdateDeadlineText(casesDomain.UpdateDeadline1h) ||
		!view.PromisedAt.Equal(now.Add(time.Hour)) ||
		view.State != casesDomain.UpdatePromisePending ||
		view.StateText != casesDomain.UpdatePromiseStateText(casesDomain.UpdatePromisePending) {
		t.Fatalf("the last promise state of the waiting view, got %+v", view)
	}
	if !strings.Contains(view.PromiseLineText, casesDomain.UpdateDeadlineText(casesDomain.UpdateDeadline1h)) ||
		!strings.Contains(view.PromiseLineText, casesDomain.UpdatePromiseStateText(casesDomain.UpdatePromisePending)) {
		t.Fatalf("the client line composes the deadline word text with the state text, got %q", view.PromiseLineText)
	}
	if len(view.DeadlineWords) != 4 {
		t.Fatalf("the form dictionary of the deadline words carries the four honest words, got %v", view.DeadlineWords)
	}

	// The honest unknown word: the promised time stays NULL (the material
	// CHECK pair owns the invariant — a fabricated timestamp NEVER
	// accompanies the unknown word), the state derives unknown with the
	// honest no-deadline text.
	second, err := promise(w.ownershipActor(), casesDomain.UpdateDeadlineUnknown, "waiting-app-promise-2")
	if err != nil {
		t.Fatal(err)
	}
	if second.PromiseDeadlineWord != casesDomain.UpdateDeadlineUnknown {
		t.Fatalf("the unknown promise outcome carries the honest word, got %+v", second)
	}
	var unknownPromisedAt *time.Time
	if err := adminPool.QueryRow(ctx, `
                select promised_at from cases.handoff_update_promises
                where tenant_id=$1 and case_id=$2 and promise_no=2`,
		w.tenant, caseID).Scan(&unknownPromisedAt); err != nil {
		t.Fatal(err)
	}
	if unknownPromisedAt != nil {
		t.Fatalf("the unknown word NEVER carries a timestamp, got %v", unknownPromisedAt)
	}
	view = viewOf()
	if view.PromiseNo != 2 || view.DeadlineWord != casesDomain.UpdateDeadlineUnknown ||
		!view.PromisedAt.IsZero() || view.State != casesDomain.UpdatePromiseUnknown {
		t.Fatalf("the unknown promise derives the honest no-deadline state, got %+v", view)
	}

	// The supersede derivation-only: the third promise of the known word
	// — the view derives №3 pending; the former rows NEVER mutate (the
	// append-only history stays, «последняя говорит»).
	third, err := promise(w.ownershipActor(), casesDomain.UpdateDeadline4h, "waiting-app-promise-3")
	if err != nil {
		t.Fatal(err)
	}
	if third.PromiseDeadlineWord != casesDomain.UpdateDeadline4h {
		t.Fatalf("the third promise outcome carries the word, got %+v", third)
	}
	view = viewOf()
	if view.PromiseNo != 3 || view.DeadlineWord != casesDomain.UpdateDeadline4h || view.State != casesDomain.UpdatePromisePending {
		t.Fatalf("the LAST promise speaks, got %+v", view)
	}
	var rowCount int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.handoff_update_promises where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID).Scan(&rowCount); err != nil {
		t.Fatal(err)
	}
	if rowCount != 3 {
		t.Fatalf("the supersede is derivation-only — the rows never mutate, got %d rows", rowCount)
	}

	// The replay of the exact repeat: the stored outcome returns (the
	// command record resolves before the exec — the reader learns the
	// SAME truth).
	replayed, err := promise(w.ownershipActor(), casesDomain.UpdateDeadline4h, "waiting-app-promise-3")
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replay || replayed.PromiseID != third.PromiseID {
		t.Fatalf("the exact repeat replays the stored outcome, got %+v", replayed)
	}

	// The same key with ANOTHER material: the honest key conflict (the
	// outcome never reproduces for another word).
	_, err = promise(w.ownershipActor(), casesDomain.UpdateDeadline1h, "waiting-app-promise-3")
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another word is the honest key conflict, got %v", err)
	}

	// The round guard of the closed lifecycle: the cancelled round
	// refuses with the store mirror words (the waiting ended honestly —
	// the promises of the closed round stay the readable history).
	cancelTurn := w.modeTurnEvent(t, caseID, "sess-waiting-app-1", "evt-waiting-app-cancel-1", `{"say":"человек не нужен"}`)
	if _, err := w.cases.CancelHumanRequest(ctx, casesAPI.CancelHumanRequestCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		TurnRef: cancelTurn, IdempotencyKey: "waiting-app-cancel-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	_, err = promise(w.ownershipActor(), casesDomain.UpdateDeadline1h, "waiting-app-closed-1")
	assertRuleError(t, err, "не принимает обещаний")
	// The closed round keeps the readable history: the last promise
	// derives closed with the honest ending word.
	view = viewOf()
	if view.ClientStatus != casesDomain.ClientWaitingCancelled || view.State != casesDomain.UpdatePromiseClosed {
		t.Fatalf("the cancelled round ends the waiting honestly, got %+v", view)
	}

	// The terminal boundary of the state dictionary: the slice carries
	// the open word only (the honest CHECK refusal — the 2.14 row-19 /
	// 2-15 precedent; the app refusal mirror stays for the future
	// dictionary growth).
	if _, err := adminPool.Exec(ctx, `
                update cases.cases set state='resolved' where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("the terminal state word is impossible in this slice (the CHECK refusal), got %v", err)
		}
	}
}
