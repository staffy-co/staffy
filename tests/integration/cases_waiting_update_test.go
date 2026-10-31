//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	casesStore "github.com/staffy-co/staffy/internal/cases/store"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The Story 2.16 step-02 probes: the migration 00016 invariants are
// pinned by the live probes immediately (the review R1 lesson of 2.7 —
// the invariant is never only a domain guard). Three probe families:
// the pg_class/RLS/grants discipline of the append-only promise table,
// the INSERT backstops (the deadline-word dictionary, the MATERIAL
// honesty pair of FR-41 — unknown ↔ NULL, the dishonest pairs of both
// directions are refused by the database, the times discipline, the
// promise number and id uniqueness, the round FK, the command/outbox
// dictionary ALTERs), and the store-level probes (the round guards with
// the mirror words of the store, the domain guards under the locks, the
// server-computed promised time, the monotonic promise numbers, the
// identifiers-only outbox envelope, the command key conflict and the
// ONE-transaction snapshot of the handoff material together with the
// promise rows).

// waitingUpdateSeedSeq builds the distinct probe identifiers of the
// promise surface (the 99800 range lives above the correction probe
// space 99400.. and the context matrix pool 99500.. — the ranges are
// unique per the 2.14 lesson).
var waitingUpdateSeedSeq int

func nextWaitingUpdateProbeID() id.UUID {
	waitingUpdateSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-7000-7000-8000-%012d", 99800+waitingUpdateSeedSeq))
}

// TestWaitingUpdateTablesGrantsAndRLS asserts the append-only discipline
// of the promise surface at the database level: RLS + FORCE, INSERT/
// SELECT for staffy_app and no UPDATE, DELETE or TRUNCATE (the
// degradation, handoff, summary and correction precedents).
func TestWaitingUpdateTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('handoff_update_promises')`)
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
	if state, ok := seen["handoff_update_promises"]; !ok || !state[0] || !state[1] {
		t.Fatalf("handoff_update_promises must carry RLS + FORCE (the append-only discipline), got %+v", state)
	}
	for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
		var allowed bool
		if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases.handoff_update_promises", grant).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if (grant == "INSERT" || grant == "SELECT") != allowed {
			t.Fatalf("%s on cases.handoff_update_promises: allowed=%v, want %v", grant, allowed, grant == "INSERT" || grant == "SELECT")
		}
	}
}

// insertUpdatePromiseRow runs the direct INSERT of one promise row over
// the live round anchor (the admin role behind the RLS curtain — the
// DB-probe precedent of 2.7..2.15). The promisedAt any: nil = NULL, a
// time = the computed server word; the dishonest pairs are the knobs.
func (w *casesWorld) insertUpdatePromiseRow(t *testing.T, caseID id.UUID, promiseID id.UUID, no, requestNo int64, deadlineWord string, promisedAt any) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.handoff_update_promises (tenant_id, case_id, promise_id, promise_no, request_no,
                        deadline_word, promised_at, actor_id, observed_at, received_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$9)`,
		w.tenant, caseID, promiseID, no, requestNo, deadlineWord, promisedAt,
		w.ownershipActor().ID, ownershipNow)
	return err
}

// TestWaitingUpdateDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00016 surface: the deadline-word dictionary (the
// four fixed words — a fabricated word never lands), the MATERIAL
// honesty pair of FR-41 (the unknown word carries NO timestamp, a known
// word carries ONLY the computed server time — both dishonest
// directions are refused by the CHECK, never only by the domain guard),
// the times discipline (received >= observed - 48h), the promise number
// and id uniqueness, the round FK (the promise anchors the request of
// THIS case) and the command/outbox dictionary ALTERs — the honest
// shape is a database guarantee, not only a domain guard.
func TestWaitingUpdateDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 968)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-waiting-db-1", "sess-waiting-db-1", `{"call":"inbound-call","from":"+79000000501"}`)
	turnRef := eventIDOf(t, w, caseID, "evt-waiting-db-1")
	// The live round anchor of the probe case (the request row №1 — the
	// FK target; the declined round №2 covers the closed history).
	if err := w.insertHandoffRow(t, caseID, handoffProbeRow{
		requestNo: 1, reason: casesDomain.HandoffReasonClientRequest,
		ownerKind: casesDomain.OwnerKindHuman, targetRoles: `["operator"]`,
		ackDeadline: casesDomain.HandoffAck1h, turnRef: turnRef, actorID: w.ownershipActor().ID,
	}); err != nil {
		t.Fatal(err)
	}

	// checkRefused pins the check_violation refusal of the dishonest row.
	checkRefused := func(probe string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the dishonest row must be refused, got accepted", probe)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("%s: expected the check_violation 23514, got %v", probe, err)
		}
	}
	// checkDuplicate pins the unique_violation refusal.
	checkDuplicate := func(probe string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the duplicate row must be refused, got accepted", probe)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23505" {
			t.Fatalf("%s: expected the unique_violation 23505, got %v", probe, err)
		}
	}
	// checkForeign pins the foreign_key_violation refusal.
	checkForeign := func(probe string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the foreign row must be refused, got accepted", probe)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23503" {
			t.Fatalf("%s: expected the foreign_key_violation 23503, got %v", probe, err)
		}
	}

	// The honest rows land: the known word with the computed time, the
	// honest unknown word with NULL (the material honesty pair).
	honestKnownID := nextWaitingUpdateProbeID()
	if err := w.insertUpdatePromiseRow(t, caseID, honestKnownID, 1, 1, casesDomain.UpdateDeadline1h, ownershipNow.Add(time.Hour)); err != nil {
		t.Fatalf("the honest known-word promise must land, got %v", err)
	}
	if err := w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 2, 1, casesDomain.UpdateDeadlineUnknown, nil); err != nil {
		t.Fatalf("the honest unknown-word promise must land, got %v", err)
	}

	// The MATERIAL honesty pair of FR-41: the unknown word NEVER carries a
	// timestamp (a fabricated time beside the honest «срок неизвестен» is
	// the database refusal), a known word NEVER lives without the computed
	// server time (the pair check refuses both directions).
	checkRefused("unknown word with a fabricated timestamp", w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 3, 1, casesDomain.UpdateDeadlineUnknown, ownershipNow.Add(time.Hour)))
	checkRefused("known word without the promised time", w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 3, 1, casesDomain.UpdateDeadline15m, nil))
	checkRefused("known word with the NULL time of a zero deadline", w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 3, 1, casesDomain.UpdateDeadline4h, nil))

	// The deadline-word dictionary: the four fixed words ONLY — a
	// fabricated word never lands (the machine never breeds a deadline).
	checkRefused("foreign deadline word", w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 3, 1, "update_2h", ownershipNow.Add(2*time.Hour)))
	checkRefused("the ack word of the 2.13 dictionary", w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 3, 1, casesDomain.HandoffAck1h, ownershipNow.Add(time.Hour)))

	// The times discipline: received >= observed - 48h (the 2.2 rule).
	_, err := adminPool.Exec(ctx, `
                insert into cases.handoff_update_promises (tenant_id, case_id, promise_id, promise_no, request_no,
                        deadline_word, promised_at, actor_id, observed_at, received_at, created_at)
                values ($1,$2,$3,3,1,$4,$5,$6,$7,$8,$8)`,
		w.tenant, caseID, nextWaitingUpdateProbeID(), casesDomain.UpdateDeadline1h, ownershipNow.Add(time.Hour),
		w.ownershipActor().ID, ownershipNow.Add(48*time.Hour), ownershipNow.Add(-time.Minute))
	checkRefused("times beyond the 48h skew", err)

	// The promise number and the promise id uniqueness (the raced
	// derivation backstops).
	checkDuplicate("raced promise number", w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 1, 1, casesDomain.UpdateDeadline15m, ownershipNow.Add(15*time.Minute)))
	checkDuplicate("duplicate promise id", w.insertUpdatePromiseRow(t, caseID, honestKnownID, 4, 1, casesDomain.UpdateDeadline15m, ownershipNow.Add(15*time.Minute)))

	// The round FK: a promise of a request that never happened (the
	// foreign round of this case) and the promise anchored to the round
	// of ANOTHER case are both the database refusal (the promise anchors
	// the request of THIS case).
	checkForeign("foreign round number", w.insertUpdatePromiseRow(t, caseID, nextWaitingUpdateProbeID(), 4, 7, casesDomain.UpdateDeadline15m, ownershipNow.Add(15*time.Minute)))
	foreignCase, _ := w.acceptedCase(t, "evt-waiting-db-2", "sess-waiting-db-2", `{"call":"inbound-call","from":"+79000000511"}`)
	checkForeign("foreign case of the round anchor", w.insertUpdatePromiseRow(t, foreignCase, nextWaitingUpdateProbeID(), 1, 1, casesDomain.UpdateDeadline15m, ownershipNow.Add(15*time.Minute)))

	// The command and outbox dictionary ALTERs: the promise record
	// command word and the promise event type are the legal dictionary
	// words (the dishonest forms are refused).
	if _, err := adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'promise_handoff_update',$3,repeat('d',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("waiting-dict-probe-%d", waitingUpdateSeedSeq), ownershipNow); err != nil {
		t.Fatalf("the promise_handoff_update command word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'promise_update',$3,repeat('d',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("waiting-dict-probe-%d", waitingUpdateSeedSeq), ownershipNow)
	checkRefused("foreign command word", err)
	if _, err := adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.update_promised',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextWaitingUpdateProbeID(), caseID, nextWaitingUpdateProbeID(), nextWaitingUpdateProbeID(), ownershipNow,
		fmt.Sprintf("waiting-outbox-probe-%d", waitingUpdateSeedSeq), ownershipNow); err != nil {
		t.Fatalf("the case.update_promised event word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.promise_recorded',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextWaitingUpdateProbeID(), caseID, nextWaitingUpdateProbeID(), nextWaitingUpdateProbeID(), ownershipNow,
		fmt.Sprintf("waiting-outbox-probe-%d", waitingUpdateSeedSeq), ownershipNow)
	checkRefused("foreign outbox event word", err)
}

// promiseParams builds the store-level command params of the promise
// probes (the deadline word is the ONLY request material — the promised
// time and the promise number are the server truths).
func promiseParams(scope tenancy.Context, actor id.UUID, caseID, conversationID id.UUID, deadlineWord, key string, now time.Time) casesPort.WorkCommandParams {
	return casesPort.WorkCommandParams{
		Scope: scope, Command: "promise_handoff_update", Key: key,
		RequestHash: strings.Repeat("e", 64), Actor: actor,
		CaseID: caseID, ConversationID: conversationID,
		PromiseDeadlineWord:  deadlineWord,
		ObservedAt:           now.Add(-time.Minute),
		ReceivedAt:           now,
		PromiseID:            nextWaitingUpdateProbeID(),
		PromiseOutboxEventID: nextWaitingUpdateProbeID(),
		CorrelationID:        nextWaitingUpdateProbeID(), Outcome: []byte(`{}`), Now: now,
	}
}

// TestWaitingUpdateStoreRecordAndRead covers the store-level surface of
// the promise record command and the composed snapshot read: the absent
// round guard (the promise never floats without the ask), the round
// guard with the mirror words of the store (the declined round consumed
// the assignment — the re-assignment decision belongs to the human),
// the domain guards under the locks (the dictionary word, the 2.2
// times), the server-computed promised time (the known word — now +
// window; the honest unknown word — NULL, the material CHECK pair), the
// monotonic promise numbers (max+1 under the locks), the
// identifiers-only outbox envelope (the deadline word travels, the
// client texts NEVER), the command key conflict (the rolled-back
// residue) and the ONE-transaction snapshot of the handoff material
// together with the promise rows (the last promise speaks read-time).
func TestWaitingUpdateStoreRecordAndRead(t *testing.T) {
	w := newCasesWorld(t, 969)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-waiting-store-1", "sess-waiting-store-1", `{"call":"inbound-call","from":"+79000000502"}`)
	conversationID := w.conversationFirst(t, caseID)
	// The declined round of the guard probes: the ask, the assignment and
	// the refusal of the assignee (the consumed round — the promise
	// refuses honestly).
	turn := w.modeTurnEvent(t, caseID, "sess-waiting-store-1", "evt-waiting-store-turn-1", `{"say":"позовите человека"}`)
	now := ownershipNow
	if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		TurnRef: turn, ExpectedModeVersion: 1,
		TargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, AckDeadline: casesDomain.HandoffAck1h,
		IdempotencyKey: "waiting-store-request-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.AssignHumanHandoff(ctx, casesAPI.AssignHumanHandoffCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		Assignee: w.backup, Backup: w.owner.ID, AckDeadline: casesDomain.HandoffAck1h,
		IdempotencyKey: "waiting-store-assign-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.DeclineHumanHandoff(ctx, casesAPI.DeclineHumanHandoffCommand{
		Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID,
		IdempotencyKey: "waiting-store-decline-1", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	// The absent round guard: the fresh case without the ask carries no
	// round to promise over (the promise never floats without the ask).
	freshCase, _ := w.acceptedCase(t, "evt-waiting-store-2", "sess-waiting-store-2", `{"call":"inbound-call","from":"+79000000503"}`)
	freshConversation := w.conversationFirst(t, freshCase)
	assertRuleError(t, store.ExecuteWorkCommand(ctx, promiseParams(scope, w.ownershipActor().ID, freshCase, freshConversation,
		casesDomain.UpdateDeadline1h, "waiting-store-absent-1", now)), "Запроса человека не существует")

	// The round guard: the declined round consumed the assignment — the
	// promise refuses with the mirror words of the store.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, promiseParams(scope, w.ownershipActor().ID, caseID, conversationID,
		casesDomain.UpdateDeadline1h, "waiting-store-declined-1", now)), "не принимает обещаний")

	// The re-assignment reopens the round (the human re-decision): the
	// promise lands over the open assigned round.
	if _, err := w.cases.AssignHumanHandoff(ctx, casesAPI.AssignHumanHandoffCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		Assignee: w.backup, Backup: w.owner.ID, AckDeadline: casesDomain.HandoffAck1h,
		IdempotencyKey: "waiting-store-assign-2", Now: now,
	}); err != nil {
		t.Fatal(err)
	}

	// The domain guards under the locks: the dictionary word (a
	// fabricated deadline NEVER passes), the 2.2 times.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, promiseParams(scope, w.ownershipActor().ID, caseID, conversationID,
		"update_2h", "waiting-store-word-1", now)), "не входит в словарь")
	futureParams := promiseParams(scope, w.ownershipActor().ID, caseID, conversationID,
		casesDomain.UpdateDeadline1h, "waiting-store-times-1", now)
	futureParams.ObservedAt = now.Add(96 * time.Hour)
	assertRuleError(t, store.ExecuteWorkCommand(ctx, futureParams), "слишком далеко в будущем")

	// The honest promise of the known word: the number 1 (the per-case
	// monotonic sequence under the locks), the promised time computed by
	// the SERVER (now + window of the word — never a request field), the
	// responsible actor of the command.
	if err := store.ExecuteWorkCommand(ctx, promiseParams(scope, w.ownershipActor().ID, caseID, conversationID,
		casesDomain.UpdateDeadline1h, "waiting-store-promise-1", now)); err != nil {
		t.Fatalf("the honest known-word promise must land, got %v", err)
	}
	var deadlineWord string
	var promiseNo, requestNo int64
	var promisedAt *time.Time
	var actor id.UUID
	if err := adminPool.QueryRow(ctx, `
                select promise_no, request_no, deadline_word, promised_at, actor_id
                from cases.handoff_update_promises
                where tenant_id=$1 and case_id=$2 and promise_no=1`,
		w.tenant, caseID).Scan(&promiseNo, &requestNo, &deadlineWord, &promisedAt, &actor); err != nil {
		t.Fatal(err)
	}
	if promiseNo != 1 || requestNo != 1 || deadlineWord != casesDomain.UpdateDeadline1h {
		t.Fatalf("the row pins the server truths, got no=%d request=%d word=%s", promiseNo, requestNo, deadlineWord)
	}
	if promisedAt == nil || !promisedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("the known word computes now + window by the server, got %v want %v", promisedAt, now.Add(time.Hour))
	}
	if actor != w.ownershipActor().ID {
		t.Fatalf("the responsible actor is the living material of the command (AC-2), got %s", actor)
	}

	// The honest unknown promise: the number 2, the NULL promised time
	// (the material honesty pair — a fabricated timestamp NEVER
	// accompanies the honest «срок неизвестен»).
	if err := store.ExecuteWorkCommand(ctx, promiseParams(scope, w.ownershipActor().ID, caseID, conversationID,
		casesDomain.UpdateDeadlineUnknown, "waiting-store-promise-2", now)); err != nil {
		t.Fatalf("the honest unknown-word promise must land, got %v", err)
	}
	if err := adminPool.QueryRow(ctx, `
                select promised_at from cases.handoff_update_promises
                where tenant_id=$1 and case_id=$2 and promise_no=2`,
		w.tenant, caseID).Scan(&promisedAt); err != nil {
		t.Fatal(err)
	}
	if promisedAt != nil {
		t.Fatalf("the unknown word carries NO promised time (the CHECK pair), got %v", promisedAt)
	}

	// The identifiers-only outbox envelope: the deadline word travels,
	// the client texts NEVER enter (the «кандидат не покидает строку»
	// perimeter of 2.6/2.13/2.15).
	var envelopePayload []byte
	if err := adminPool.QueryRow(ctx, `
                select payload from cases.outbox_events
                where tenant_id=$1 and event_type='case.update_promised'
                order by created_at, event_id limit 1`,
		w.tenant).Scan(&envelopePayload); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(envelopePayload, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tenant_id", "case_id", "request_no", "promise_id", "promise_no", "deadline_word", "actor_id", "occurred_at"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("the envelope carries the identifier/word key %s, got %s", key, envelopePayload)
		}
	}
	if envelope["deadline_word"] != casesDomain.UpdateDeadline1h || envelope["promise_no"] != float64(1) {
		t.Fatalf("the envelope carries the deadline word and the promise number, got %s", envelopePayload)
	}
	if strings.Contains(string(envelopePayload), "Обновление обещано") || strings.Contains(string(envelopePayload), "выдуманного") {
		t.Fatalf("the client texts NEVER enter the envelope, got %s", envelopePayload)
	}

	// The command key conflict: the same key with ANOTHER material is the
	// honest conflict (the idempotency surface of 2.2..2.15 — the exec
	// rolls back, no residue lands).
	keyConflict := store.ExecuteWorkCommand(ctx, promiseParams(scope, w.ownershipActor().ID, caseID, conversationID,
		casesDomain.UpdateDeadline4h, "waiting-store-promise-1", now))
	if !errors.Is(keyConflict, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material is the command key conflict, got %v", keyConflict)
	}
	var promiseCount int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.handoff_update_promises where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID).Scan(&promiseCount); err != nil {
		t.Fatal(err)
	}
	if promiseCount != 2 {
		t.Fatalf("the rolled-back conflict leaves no residue, got %d rows", promiseCount)
	}

	// The ONE-transaction snapshot: the handoff material together with
	// the promise rows (the consistent read of the waiting surface).
	snapshot, err := store.ReadUpdatePromiseState(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Handoff.Requests) != 1 || len(snapshot.Handoff.Events) != 3 {
		t.Fatalf("the snapshot reads the lifecycle material, got %+v", snapshot.Handoff)
	}
	if len(snapshot.Promises) != 2 {
		t.Fatalf("the snapshot reads the promise rows, got %d", len(snapshot.Promises))
	}
	if snapshot.Promises[0].PromiseNo != 1 || snapshot.Promises[0].DeadlineWord != casesDomain.UpdateDeadline1h ||
		!snapshot.Promises[0].PromisedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("the rows are ordered by the promise numbers, got %+v", snapshot.Promises[0])
	}
	if snapshot.Promises[1].DeadlineWord != casesDomain.UpdateDeadlineUnknown || !snapshot.Promises[1].PromisedAt.IsZero() {
		t.Fatalf("the unknown row carries the zero time (the NULL of the table), got %+v", snapshot.Promises[1])
	}
	// The read-time derivation over the snapshot material: the LAST
	// promise of the round speaks (№2 unknown), the former promise
	// derives superseded — the moment NEVER mutates the rows.
	roundPromises := []casesDomain.UpdatePromiseRecord{}
	for _, record := range snapshot.Promises {
		if record.RequestNo == snapshot.Handoff.Requests[0].RequestNo {
			roundPromises = append(roundPromises, record)
		}
	}
	view := casesDomain.DeriveUpdatePromiseState(roundPromises, casesDomain.HandoffStateAssigned, now)
	if !view.HasPromise || view.PromiseNo != 2 || view.DeadlineWord != casesDomain.UpdateDeadlineUnknown || view.State != casesDomain.UpdatePromiseUnknown {
		t.Fatalf("the LAST promise of the round speaks with the honest unknown state, got %+v", view)
	}
	if got := casesDomain.DeriveUpdatePromiseRowState(false, roundPromises[0].DeadlineWord, roundPromises[0].PromisedAt, casesDomain.HandoffStateAssigned, now); got != casesDomain.UpdatePromiseSuperseded {
		t.Fatalf("the former promise derives superseded, got %q", got)
	}
}
