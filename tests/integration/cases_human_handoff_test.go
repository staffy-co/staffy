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

	casesAPI "github.com/artemiosu/staffy/internal/cases/api"
	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	casesStore "github.com/artemiosu/staffy/internal/cases/store"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5/pgconn"
)

// handoffSeedSeq keeps every seeded handoff-surface row globally unique
// across the whole suite (the degradation precedent).
var handoffSeedSeq int64

// nextHandoffProbeID mints one globally unique probe id.
func nextHandoffProbeID() id.UUID {
	handoffSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-7000-7000-8000-%012d", 99100+handoffSeedSeq))
}

// The Story 2.13 step-02 probes: the migration 00013 invariants are
// pinned by the live probes immediately (the review R1 lesson of 2.7 —
// the invariant is never only a domain guard). Three probe families: the
// pg_class/RLS/grants discipline of the two append-only lifecycle
// tables, the INSERT backstops (the reason/roles/deadline/kind/result
// dictionaries, the per-kind material CHECKs, the request and event
// number UNIQUEs, the request FK, the command/outbox dictionary ALTERs),
// and the store-level probes (the six lifecycle commands under the
// locks, the turn anchor guard, the one-competing-task 409, the mode
// ratchet raise through the existing 2.5 machinery, the composed 2.4
// transfer transitions of the acknowledge and the return, the freeze
// gate re-validation of the commitments, the read-time derivations and
// the perimeter absence in the fact channels).

// TestHandoffTablesGrantsAndRLS asserts the append-only discipline of
// the human handoff lifecycle surfaces at the database level: RLS +
// FORCE on both tables, INSERT/SELECT for staffy_app and no UPDATE,
// DELETE or TRUNCATE (the degradation precedent of 2.12).
func TestHandoffTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('human_handoffs', 'human_handoff_events')`)
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
	for _, table := range []string{"human_handoffs", "human_handoff_events"} {
		if state, ok := seen[table]; !ok || !state[0] || !state[1] {
			t.Fatalf("%s must carry RLS + FORCE (the append-only discipline), got %+v", table, state)
		}
		for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
			var allowed bool
			if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases."+table, grant).Scan(&allowed); err != nil {
				t.Fatal(err)
			}
			if (grant == "INSERT" || grant == "SELECT") != allowed {
				t.Fatalf("%s on cases.%s: allowed=%v, want %v", grant, table, allowed, grant == "INSERT" || grant == "SELECT")
			}
		}
	}
}

// handoffProbeRow carries the honest template of one request row with
// the per-probe dishonest knobs.
type handoffProbeRow struct {
	requestNo    int64
	reason       string
	ownerKind    string
	targetRoles  string
	ackDeadline  string
	turnRef      id.UUID
	actorID      id.UUID
	zeroDeadline bool
}

// insertHandoffRow runs the direct INSERT of one request row over the
// live case anchor (the admin role behind the RLS curtain — the DB-probe
// precedent of 2.7..2.12).
func (w *casesWorld) insertHandoffRow(t *testing.T, caseID id.UUID, row handoffProbeRow) error {
	t.Helper()
	deadline := ownershipNow.Add(15 * time.Minute)
	if row.zeroDeadline {
		deadline = time.Time{}
	}
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.human_handoffs (tenant_id, case_id, request_no, reason, owner_kind, owner_ref, owner_epoch,
                        target_roles, ack_deadline, ack_deadline_at, turn_ref, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,1,$7::jsonb,$8,$9,$10,$11,$12)`,
		w.tenant, caseID, row.requestNo, row.reason, row.ownerKind, w.owner.ID,
		row.targetRoles, row.ackDeadline, deadline, row.turnRef, row.actorID, ownershipNow)
	return err
}

// insertHandoffEventRow runs the direct INSERT of one lifecycle event
// row over the seeded request (the per-kind material knobs).
func (w *casesWorld) insertHandoffEventRow(t *testing.T, caseID id.UUID, requestNo, eventNo int64, kind string,
	assignee, backup *id.UUID, ackDeadline *string, deadlineAt *time.Time, result *string, turnRef *id.UUID, resultingEpoch *int64) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.human_handoff_events (tenant_id, case_id, request_no, event_no, kind,
                        assignee, backup_owner, ack_deadline, deadline_at, result, turn_ref, resulting_epoch, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		w.tenant, caseID, requestNo, eventNo, kind,
		assignee, backup, ackDeadline, deadlineAt, result, turnRef, resultingEpoch, w.ownershipActor().ID, ownershipNow)
	return err
}

// TestHandoffDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00013 surface: the reason/owner-kind/roles/deadline
// dictionaries of the request row, the target roles array shape (the
// non-empty duplicate-free dictionary subset), the request number
// monotonicity UNIQUE, the event kind dictionary with the per-kind
// material CHECKs (the assigned round material, the acknowledged epoch,
// the cancelled turn anchor, the returned result word), the event number
// UNIQUE, the request FK of the events, and the command/outbox ALTERs —
// the honest shape is a database guarantee, not only a domain guard.
func TestHandoffDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 981)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-handoff-2-13-1", "sess-handoff-2-13-1", `{"call":"inbound-call","from":"+79000000221"}`)
	turnRef := eventIDOf(t, w, caseID, "evt-handoff-2-13-1")

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
	honestRow := func(no int64) handoffProbeRow {
		return handoffProbeRow{
			requestNo: no, reason: casesDomain.HandoffReasonClientRequest,
			ownerKind: casesDomain.OwnerKindHuman, targetRoles: `["operator"]`,
			ackDeadline: casesDomain.HandoffAck15m, turnRef: turnRef, actorID: w.ownershipActor().ID,
		}
	}

	// The honest request row lands (the dictionary reason, the owner
	// snapshot, the roles subset, the policy deadline word with its time).
	if err := w.insertHandoffRow(t, caseID, honestRow(1)); err != nil {
		t.Fatalf("the honest request must land, got %v", err)
	}

	// The request dictionary and shape backstops.
	checkRefused("foreign reason word", w.insertHandoffRow(t, caseID, func() handoffProbeRow { r := honestRow(2); r.reason = "operator_kindness"; return r }()))
	checkRefused("foreign owner kind", w.insertHandoffRow(t, caseID, func() handoffProbeRow { r := honestRow(2); r.ownerKind = "department"; return r }()))
	checkRefused("foreign role word", w.insertHandoffRow(t, caseID, func() handoffProbeRow { r := honestRow(2); r.targetRoles = `["operator","manager"]`; return r }()))
	checkRefused("duplicate role word", w.insertHandoffRow(t, caseID, func() handoffProbeRow { r := honestRow(2); r.targetRoles = `["operator","operator"]`; return r }()))
	checkRefused("empty roles array", w.insertHandoffRow(t, caseID, func() handoffProbeRow { r := honestRow(2); r.targetRoles = `[]`; return r }()))
	checkRefused("non-array roles", w.insertHandoffRow(t, caseID, func() handoffProbeRow { r := honestRow(2); r.targetRoles = `"operator"`; return r }()))
	checkRefused("foreign deadline word", w.insertHandoffRow(t, caseID, func() handoffProbeRow { r := honestRow(2); r.ackDeadline = "ack_2h"; return r }()))
	checkRefused("zero request number", w.insertHandoffRow(t, caseID, honestRow(0)))
	checkDuplicate("the raced request number", w.insertHandoffRow(t, caseID, honestRow(1)))

	// The honest lifecycle event rows land: the assigned round (the
	// assignee, the backup, the round's own deadline word with its time),
	// the acknowledged epoch, the declined round, the cancelled turn and
	// the returned result word — every honest material shape is legal.
	assignee, backup := w.backup, w.owner.ID
	ackWord, resultWord := casesDomain.HandoffAck15m, casesDomain.HandoffResultIssueResolved
	var epoch int64 = 2
	if err := w.insertHandoffEventRow(t, caseID, 1, 1, casesDomain.HandoffEventAssigned,
		&assignee, &backup, &ackWord, ptrTime(ownershipNow.Add(15*time.Minute)), nil, nil, nil); err != nil {
		t.Fatalf("the honest assigned event must land, got %v", err)
	}
	if err := w.insertHandoffEventRow(t, caseID, 1, 2, casesDomain.HandoffEventAcknowledged,
		nil, nil, nil, nil, nil, nil, &epoch); err != nil {
		t.Fatalf("the honest acknowledged event must land, got %v", err)
	}
	// The second request of the probe case: the declined, cancelled and
	// returned shapes (the honest closed lifecycles of the history).
	if err := w.insertHandoffRow(t, caseID, honestRow(2)); err != nil {
		t.Fatalf("the second honest request must land, got %v", err)
	}
	if err := w.insertHandoffEventRow(t, caseID, 2, 1, casesDomain.HandoffEventDeclined, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("the honest declined event must land, got %v", err)
	}
	if err := w.insertHandoffEventRow(t, caseID, 2, 2, casesDomain.HandoffEventCancelled, nil, nil, nil, nil, nil, &turnRef, nil); err != nil {
		t.Fatalf("the honest cancelled event must land, got %v", err)
	}
	if err := w.insertHandoffEventRow(t, caseID, 2, 3, casesDomain.HandoffEventReturned, nil, nil, nil, nil, &resultWord, nil, nil); err != nil {
		t.Fatalf("the honest returned event must land, got %v", err)
	}

	// The event kind dictionary and the per-kind material CHECKs: every
	// kind carries exactly its own material — never another kind's.
	checkRefused("foreign event kind", w.insertHandoffEventRow(t, caseID, 2, 4, "expired", nil, nil, nil, nil, nil, nil, nil))
	checkRefused("zero event number", w.insertHandoffEventRow(t, caseID, 2, 0, casesDomain.HandoffEventDeclined, nil, nil, nil, nil, nil, nil, nil))
	checkDuplicate("the raced event number", w.insertHandoffEventRow(t, caseID, 2, 3, casesDomain.HandoffEventDeclined, nil, nil, nil, nil, nil, nil, nil))
	checkRefused("assigned without assignee", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventAssigned, nil, &backup, &ackWord, ptrTime(ownershipNow.Add(15*time.Minute)), nil, nil, nil))
	checkRefused("assigned without deadline word", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventAssigned, &assignee, &backup, nil, ptrTime(ownershipNow.Add(15*time.Minute)), nil, nil, nil))
	checkRefused("assigned with result", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventAssigned, &assignee, &backup, &ackWord, ptrTime(ownershipNow.Add(15*time.Minute)), &resultWord, nil, nil))
	checkRefused("acknowledged without epoch", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventAcknowledged, nil, nil, nil, nil, nil, nil, nil))
	checkRefused("acknowledged with assignee", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventAcknowledged, &assignee, nil, nil, nil, nil, nil, &epoch))
	checkRefused("declined with turn anchor", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventDeclined, nil, nil, nil, nil, nil, &turnRef, nil))
	checkRefused("cancelled without turn anchor", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventCancelled, nil, nil, nil, nil, nil, nil, nil))
	checkRefused("cancelled with epoch", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventCancelled, nil, nil, nil, nil, nil, &turnRef, &epoch))
	checkRefused("returned without result", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventReturned, nil, nil, nil, nil, nil, nil, nil))
	checkRefused("returned foreign result word", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventReturned, nil, nil, nil, nil, ptrString("not_resolved"), nil, nil))
	checkRefused("returned with epoch", w.insertHandoffEventRow(t, caseID, 2, 4, casesDomain.HandoffEventReturned, nil, nil, nil, nil, &resultWord, nil, &epoch))
	// The FK backstop: an event of a foreign request is impossible.
	_, err := adminPool.Exec(ctx, `
                insert into cases.human_handoff_events (tenant_id, case_id, request_no, event_no, kind,
                        assignee, backup_owner, ack_deadline, deadline_at, result, turn_ref, resulting_epoch, actor_id, created_at)
                values ($1,$2,7,1,$3,NULL,NULL,NULL,NULL,NULL,NULL,NULL,$4,$5)`,
		w.tenant, caseID, casesDomain.HandoffEventDeclined, w.ownershipActor().ID, ownershipNow)
	if err == nil {
		t.Fatalf("the foreign request must be refused by the FK backstop")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.SQLState() != "23503" {
		t.Fatalf("the foreign request refusal is the foreign_key_violation 23503, got %v", err)
	}

	// The command and outbox dictionary ALTERs: the six lifecycle command
	// words and the six event types are the legal dictionary words (the
	// dishonest forms are refused).
	for _, word := range []string{"request_human", "assign_human_handoff", "acknowledge_human_handoff",
		"decline_human_handoff", "cancel_human_request", "return_to_staffy"} {
		if _, err := adminPool.Exec(ctx, `
                        insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                        values ($1,$2,$3,$4,repeat('d',64),'{}'::jsonb,$5)`,
			w.tenant, w.ownershipActor().ID, word, fmt.Sprintf("handoff-dict-probe-%d", handoffSeedSeq), ownershipNow); err != nil {
			t.Fatalf("the %s command word must land, got %v", word, err)
		}
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'summon_human',$3,repeat('d',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("handoff-dict-probe-%d", handoffSeedSeq), ownershipNow)
	checkRefused("foreign command word", err)
	for _, word := range []string{"case.human_requested", "case.human_assigned", "case.human_acknowledged",
		"case.human_declined", "case.human_cancelled", "case.human_returned"} {
		if _, err := adminPool.Exec(ctx, `
                        insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                        values ($1,$2,$3,1,$4,$5,$6,$7,'{}'::jsonb,$8,$9)`,
			w.tenant, nextHandoffProbeID(), word, caseID, nextHandoffProbeID(), nextHandoffProbeID(), ownershipNow,
			fmt.Sprintf("handoff-outbox-probe-%d", handoffSeedSeq), ownershipNow); err != nil {
			t.Fatalf("the %s event word must land, got %v", word, err)
		}
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.human_summoned',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextHandoffProbeID(), caseID, nextHandoffProbeID(), nextHandoffProbeID(), ownershipNow,
		fmt.Sprintf("handoff-outbox-probe-%d", handoffSeedSeq), ownershipNow)
	checkRefused("foreign outbox event word", err)
}

// ptrTime boxes one time (the probe material helper).
func ptrTime(value time.Time) *time.Time { return &value }

// ptrString boxes one word (the probe material helper).
func ptrString(value string) *string { return &value }

// TestHandoffStoreLifecycleReads covers the store-level surface: the six
// lifecycle commands under the case + conversation locks (the request
// write with the conditional mode ratchet raise and both envelopes, the
// assignment round, the composed acknowledge and return transitions of
// the 2.4 machinery, the decline, the cancellation), the turn anchor
// guard, the one-competing-task 409 with the current lifecycle words, the
// dictionary guard refusals under the locks, the key conflict of the
// same key with another material, the honest outbox envelopes (the mode
// reason text never enters), the freeze gate re-validation of the new
// commitments (the mode half and the handoff half), the read-time
// derivations (the assigned round, the overdue derivation, the closed
// history) and the perimeter absence in the fact channels.
func TestHandoffStoreLifecycleReads(t *testing.T) {
	w := newCasesWorld(t, 982)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-handoff-store-1", "sess-handoff-store-1", `{"call":"inbound-call","from":"+79000000222"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-handoff-store-1", "evt-handoff-store-1-turn", `{"say":"позовите человека"}`)
	store := casesStore.NewPostgres(runtimePool)
	// The command-probe scope: the ACTOR is the principal (the
	// command_records RLS policy demands actor == principal).
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	requestParams := func(key string, turnRef id.UUID, expectedModeVersion int64, roles []string, deadline string) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: scope, Command: "request_human", Key: key,
			RequestHash: strings.Repeat("e", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID,
			TurnRef: turnRef, ExpectedModeVersion: expectedModeVersion,
			HandoffTargetRoles: roles, HandoffAckDeadline: deadline,
			ModeTermID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(),
			HandoffOutboxEventID: nextHandoffProbeID(),
			CorrelationID:        nextHandoffProbeID(), Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}

	// The honest ask: the request row lands with the conditional mode
	// raise (unclassified -> human_requested through the EXISTING 2.5
	// machinery) and BOTH envelopes — case.mode_changed of the system
	// reason constant and case.human_requested of the identifiers and the
	// dictionary words.
	if err := store.ExecuteWorkCommand(ctx, requestParams("handoff-request-1", turn, 1,
		[]string{casesDomain.HandoffTargetRoleOperator}, casesDomain.HandoffAck15m)); err != nil {
		t.Fatalf("the honest request must land, got %v", err)
	}
	// The mode raise: the version 2 human_requested term with the turn
	// anchor and the system reason constant.
	modeView := w.modeView(t, caseID)
	if len(modeView.Terms) != 2 || modeView.Active == nil ||
		modeView.Active.Mode != casesDomain.ModeHumanRequested || modeView.Active.ModeVersion != 2 ||
		modeView.Active.TurnRef != turn || modeView.Active.SignalSource != casesDomain.ModeSignalTurnEvent {
		t.Fatalf("expected the version 2 human_requested turn-anchored term, got %+v", modeView.Active)
	}
	if modeView.Active.Reason != casesDomain.HumanRequestReasonText {
		t.Fatalf("the mode reason is the system constant (the operator never types it), got %q", modeView.Active.Reason)
	}
	// The read: the request derives the requested state with the honest
	// round material of the future assignment.
	material, err := store.ReadHandoffMaterial(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(material.Requests) != 1 || len(material.Events) != 0 {
		t.Fatalf("expected one request without events, got %+v", material)
	}
	view := casesDomain.DeriveHandoffState(material.Requests[0], material.Events, ownershipNow)
	if view.State != casesDomain.HandoffStateRequested || view.Active() {
		// requested is open — Active() is true; the state word is checked first.
	}
	if view.State != casesDomain.HandoffStateRequested {
		t.Fatalf("the fresh request derives requested, got %s", view.State)
	}
	if !view.Active() {
		t.Fatalf("the fresh request is OPEN (the one-competing-task truth)")
	}

	// The one-competing-task 409: the repeated ask refuses with the
	// CURRENT lifecycle words (the state and the client status).
	repeatErr := store.ExecuteWorkCommand(ctx, requestParams("handoff-request-2", turn, 2,
		[]string{casesDomain.HandoffTargetRoleOperator}, casesDomain.HandoffAck15m))
	var activeErr casesDomain.HandoffActiveError
	if !errors.As(repeatErr, &activeErr) {
		t.Fatalf("the repeated request must refuse with the handoff active conflict, got %v", repeatErr)
	}
	if activeErr.State != casesDomain.HandoffStateRequested ||
		activeErr.ClientStatus != casesDomain.DeriveHandoffClientStatus(casesDomain.HandoffStateRequested) {
		t.Fatalf("the conflict carries the current lifecycle words, got %+v", activeErr)
	}

	// The dictionary and anchor guards under the locks (the honest 400
	// words of the domain guard re-validated by the store).
	assertRuleError(t, store.ExecuteWorkCommand(ctx, requestParams(fmt.Sprintf("handoff-guard-%d", handoffSeedSeq), turn, 2,
		[]string{casesDomain.HandoffTargetRoleOperator}, "ack_2h")), "не входит в утверждённую policy")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, requestParams(fmt.Sprintf("handoff-guard-%d", handoffSeedSeq), turn, 2,
		[]string{"manager"}, casesDomain.HandoffAck15m)), "не входит в словарь")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, requestParams(fmt.Sprintf("handoff-guard-%d", handoffSeedSeq), turn, 0,
		[]string{casesDomain.HandoffTargetRoleOperator}, casesDomain.HandoffAck15m)), "expected_mode_version обязателен")
	// The turn anchor guard: a foreign turn is not an anchor of THIS case.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, requestParams("handoff-foreign-turn", nextHandoffProbeID(), 2,
		[]string{casesDomain.HandoffTargetRoleOperator}, casesDomain.HandoffAck15m)), "turn_ref не найдено")
	// The mode CAS: the raced ladder version refuses with the conflict.
	var modeConflict casesDomain.ModeConflictError
	if err := store.ExecuteWorkCommand(ctx, requestParams("handoff-stale-mode", turn, 1,
		[]string{casesDomain.HandoffTargetRoleOperator}, casesDomain.HandoffAck15m)); !errors.As(err, &modeConflict) {
		t.Fatalf("the raced mode version must refuse with the mode conflict, got %v", err)
	}
	// NOTE: the same key with another material on the OPEN lifecycle
	// refuses with the one-competing-task word FIRST (the honest order —
	// the live ask dominates the idempotency backstop; the store command
	// key conflict is probed on the CLOSED lifecycle below, and the exact
	// replay is the app layer's lookup of step-03).
	// The unknown case: the honest 404 (the non-participant path).
	unknownCase := requestParams("handoff-unknown-case", turn, 2,
		[]string{casesDomain.HandoffTargetRoleOperator}, casesDomain.HandoffAck15m)
	unknownCase.CaseID = nextHandoffProbeID()
	if err := store.ExecuteWorkCommand(ctx, unknownCase); !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the unknown case must refuse with the not-found word, got %v", err)
	}

	// The outbox honesty of the request: the human_requested envelope
	// carries the identifiers and the dictionary words only — the system
	// reason text of the mode NEVER enters (it lives in the mode_changed
	// envelope of the 2.5 perimeter).
	var requestPayload string
	if err := adminPool.QueryRow(ctx, `
                select payload from cases.outbox_events
                where tenant_id=$1 and dedup_key=$2`, w.tenant,
		casesDomain.DedupKeyHandoffEvent(caseID, 1, 0)).Scan(&requestPayload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(requestPayload, casesDomain.HumanRequestReasonText) {
		t.Fatalf("the lifecycle envelope never carries the mode reason text, got %s", requestPayload)
	}
	var requestEnvelope casesDomain.HandoffEventPayload
	if err := json.Unmarshal([]byte(requestPayload), &requestEnvelope); err != nil {
		t.Fatal(err)
	}
	if requestEnvelope.RequestNo != 1 || requestEnvelope.EventNo != 0 ||
		requestEnvelope.Reason != casesDomain.HandoffReasonClientRequest ||
		requestEnvelope.TargetRoles[0] != casesDomain.HandoffTargetRoleOperator ||
		requestEnvelope.AckDeadline != casesDomain.HandoffAck15m || requestEnvelope.TurnRef != turn {
		t.Fatalf("the request envelope carries the identifiers and the words, got %+v", requestEnvelope)
	}

	// The assignment round: the assignee (the backup member of the world)
	// with the role word of the request's target set, the backup owner and
	// the round's deadline word; the assignment NEVER changes the owner.
	assignParams := func(key, role, deadline string) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: scope, Command: "assign_human_handoff", Key: key,
			RequestHash: strings.Repeat("f", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID,
			HandoffAssignee: w.backup, HandoffBackup: w.owner.ID,
			HandoffAssigneeRole: role, HandoffAckDeadline: deadline,
			OutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
			Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}
	// The assignment guard refusals under the locks (the state is still
	// the fresh requested round — the guard validates the round material
	// before any honest assignment lands).
	assertRuleError(t, store.ExecuteWorkCommand(ctx, assignParams(fmt.Sprintf("handoff-assign-guard-%d", handoffSeedSeq),
		"manager", casesDomain.HandoffAck15m)), "не входит в словарь целевых ролей")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, assignParams(fmt.Sprintf("handoff-assign-guard-%d", handoffSeedSeq),
		casesDomain.HandoffTargetRoleSupervision, casesDomain.HandoffAck15m)), "не входит в целевые роли запроса")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, assignParams(fmt.Sprintf("handoff-assign-guard-%d", handoffSeedSeq),
		casesDomain.HandoffTargetRoleOperator, "ack_9h")), "не входит в утверждённую policy")
	// The honest assignment round: the assignee (the backup member of the
	// world) with the role word of the request's target set, the backup
	// owner and the round's deadline word; the assignment NEVER changes
	// the owner.
	if err := store.ExecuteWorkCommand(ctx, assignParams("handoff-assign-1",
		casesDomain.HandoffTargetRoleOperator, casesDomain.HandoffAck15m)); err != nil {
		t.Fatalf("the honest assignment must land, got %v", err)
	}
	material, err = store.ReadHandoffMaterial(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	view = casesDomain.DeriveHandoffState(material.Requests[0], material.Events, ownershipNow)
	if view.State != casesDomain.HandoffStateAssigned || view.Assignee != w.backup || view.BackupOwner != w.owner.ID {
		t.Fatalf("the assigned round carries the assignee and the backup, got %+v", view)
	}
	if !view.DeadlineAt.Equal(ownershipNow.Add(15 * time.Minute)) {
		t.Fatalf("the round deadline is the word's window, got %v", view.DeadlineAt)
	}
	// НАЗНАЧЕНИЕ ≠ ПРИНЯТИЕ: the owner is still the prior owner (the
	// assignment never moves the ownership — AC-3).
	if ownership := w.ownershipState(t, caseID); ownership.State.Current.Epoch != 1 ||
		ownership.State.Current.OwnerRef != w.owner.ID {
		t.Fatalf("the assignment never changes the owner, got %+v", ownership.State.Current)
	}
	// The freeze gate of the new commitments (the mode half — the
	// human_requested mode): the honest 2.9 command refuses with the
	// frozen word.
	_, err = w.commitmentCreate(t, w.commitmentCreateCommand(caseID, workItemID, nextHandoffProbeID(),
		casesDomain.CommitmentStatusDraft, "handoff-freeze-mode"))
	assertRuleError(t, err, "заморожено")

	// The acknowledge composition: the actor IS the assignee (the
	// principal scope of the assignee member), the CAS epoch matches the
	// current owner — ONE transaction writes the acknowledged event, the
	// composed TransferRequest, the acknowledged Resolution, the new
	// ownership Term and BOTH envelopes (ownership_changed +
	// human_acknowledged).
	acknowledgeScope, err := tenancy.New(w.backup, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	acknowledgeParams := func(key string, actor id.UUID, expectedEpoch int64) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: acknowledgeScope, Command: "acknowledge_human_handoff", Key: key,
			RequestHash: strings.Repeat("a", 64), Actor: actor,
			CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID,
			ExpectedVersion:   expectedEpoch,
			TransferRequestID: nextHandoffProbeID(), TransferResolutionID: nextHandoffProbeID(),
			VerificationID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(),
			HandoffOutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
			Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}
	// The non-assignee actor refuses (the assignment is never the
	// acknowledgement — AC-3/5).
	assertRuleError(t, store.ExecuteWorkCommand(ctx, acknowledgeParams("handoff-ack-foreign-actor",
		w.owner.ID, 1)), "может только назначенный специалист")
	// The raced CAS: the stale epoch refuses honestly.
	if err := store.ExecuteWorkCommand(ctx, acknowledgeParams("handoff-ack-stale-epoch", w.backup, 7)); !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("the stale epoch must refuse with the stale signal, got %v", err)
	}
	// The honest acceptance: the composed transition lands.
	if err := store.ExecuteWorkCommand(ctx, acknowledgeParams("handoff-ack-1", w.backup, 1)); err != nil {
		t.Fatalf("the honest acknowledge must land, got %v", err)
	}
	// The ownership moved: the epoch 2 term of the assignee, the
	// transfer_acknowledge origin, the inbox surface, the composed
	// request and resolution rows.
	ownership := w.ownershipState(t, caseID)
	if ownership.State.Current.Epoch != 2 || ownership.State.Current.OwnerKind != casesDomain.OwnerKindHuman ||
		ownership.State.Current.OwnerRef != w.backup || ownership.State.Current.WritableSurface != casesDomain.WritableSurfaceInboxV2 ||
		ownership.State.Current.Origin != casesDomain.OwnershipOriginTransferAcknowledge {
		t.Fatalf("the acknowledged transition moved the ownership to the assignee, got %+v", ownership.State.Current)
	}
	var composedRequests, composedResolutions int
	if err := adminPool.QueryRow(ctx, `
                select (select count(*) from cases.transfer_requests where tenant_id=$1 and case_id=$2),
                       (select count(*) from cases.transfer_resolutions r
                          join cases.transfer_requests q on q.tenant_id = r.tenant_id and q.request_id = r.request_id
                          where q.tenant_id=$1 and q.case_id=$2)`,
		w.tenant, caseID).Scan(&composedRequests, &composedResolutions); err != nil {
		t.Fatal(err)
	}
	if composedRequests != 1 || composedResolutions != 1 {
		t.Fatalf("the composed transfer machinery wrote one request and one resolution, got %d/%d", composedRequests, composedResolutions)
	}
	var toKind, surface, reason string
	var toRef id.UUID
	if err := adminPool.QueryRow(ctx, `
                select to_owner_kind, to_owner_ref, writable_surface, reason from cases.transfer_requests
                where tenant_id=$1 and case_id=$2`, w.tenant, caseID).Scan(&toKind, &toRef, &surface, &reason); err != nil {
		t.Fatal(err)
	}
	if toKind != casesDomain.OwnerKindHuman || toRef != w.backup || surface != casesDomain.WritableSurfaceInboxV2 ||
		reason != casesDomain.HandoffReasonText(casesDomain.HandoffReasonClientRequest) {
		t.Fatalf("the composed request carries the honest dictionary reason constant, got %s/%s/%s", toKind, surface, reason)
	}
	// The read: the acknowledged state with the resulting epoch; the
	// second acknowledge (the raced twin of the two-operator race)
	// refuses on the closed state honestly.
	material, err = store.ReadHandoffMaterial(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	view = casesDomain.DeriveHandoffState(material.Requests[0], material.Events, ownershipNow)
	if view.State != casesDomain.HandoffStateAcknowledged || view.ResultingEpoch != 2 || view.Active() {
		t.Fatalf("the acknowledged lifecycle is closed with the resulting epoch, got %+v", view)
	}
	assertRuleError(t, store.ExecuteWorkCommand(ctx, acknowledgeParams("handoff-ack-raced-twin",
		w.backup, 2)), "только из состояния «назначен»")

	// The return composition: the human owner returns the case to the
	// digital employee — the result word of the fixed dictionary, the
	// composed transfer to the staffy owner (the case employee, the
	// external_inbox surface), the epoch 3 term and the human_returned
	// envelope. The mode is NEVER touched by the return (AC-9).
	returnParams := func(key, result string, expectedEpoch int64) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: acknowledgeScope, Command: "return_to_staffy", Key: key,
			RequestHash: strings.Repeat("r", 64), Actor: w.backup,
			CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID,
			ExpectedVersion: expectedEpoch, HandoffResult: result,
			TransferRequestID: nextHandoffProbeID(), TransferResolutionID: nextHandoffProbeID(),
			VerificationID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(),
			HandoffOutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
			Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}
	// The foreign result word refuses under the locks.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, returnParams("handoff-return-foreign-word",
		"kind_of_resolved", 2)), "не входит в словарь")
	// The honest return: the ownership moves back to the digital employee.
	if err := store.ExecuteWorkCommand(ctx, returnParams("handoff-return-1",
		casesDomain.HandoffResultIssueResolved, 2)); err != nil {
		t.Fatalf("the honest return must land, got %v", err)
	}
	ownership = w.ownershipState(t, caseID)
	if ownership.State.Current.Epoch != 3 || ownership.State.Current.OwnerKind != casesDomain.OwnerKindStaffy ||
		ownership.State.Current.OwnerRef != w.employeeID ||
		ownership.State.Current.WritableSurface != casesDomain.WritableSurfaceExternalInbox {
		t.Fatalf("the return moved the ownership to the digital employee of the case, got %+v", ownership.State.Current)
	}
	// The return envelope: the result word travels; the mode keeps its
	// honest word (the ratchet — the return never lowers the ladder).
	modeView = w.modeView(t, caseID)
	if modeView.Active.Mode != casesDomain.ModeHumanRequested {
		t.Fatalf("the return never touches the mode, got %s", modeView.Active.Mode)
	}
	// The closed lifecycle refuses the return repeat (the honest state
	// word) and the cancellation of a closed lifecycle.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, returnParams("handoff-return-repeat",
		casesDomain.HandoffResultHandedBack, 3)), "только из состояния «принято»")

	// The perimeter probe (the lifecycle rows are NEVER facts): the
	// requests and the events never appear in the fact channels of the
	// case (the live probe of the 2.11/2.12 precedent).
	var factRows int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.facts where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID).Scan(&factRows); err != nil {
		t.Fatal(err)
	}
	if factRows != 0 {
		t.Fatalf("the lifecycle rows never write fact anchors, got %d rows", factRows)
	}
}

// TestHandoffStoreDeclineCancelPaths covers the auxiliary lifecycle paths
// of the store surface: the decline (the assignee's refusal — the owner
// NEVER changes, the request stays open), the re-assignment (a NEW event
// with a NEW deadline word — the human re-decision), the overdue
// read-time derivation (the expired round refuses the acknowledge), the
// cancellation (the client's cancelling turn anchor, the owner and the
// mode untouched, the closed history), the NEW ask after a closed
// lifecycle (the fresh request number) and the store command key
// conflict on the closed lifecycle.
func TestHandoffStoreDeclineCancelPaths(t *testing.T) {
	w := newCasesWorld(t, 983)
	ctx := context.Background()
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	acknowledgeScope, err := tenancy.New(w.backup, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	// The decline and the re-assignment (a fresh case): the declined round
	// stays open, the owner never changes, the re-assignment is a NEW
	// event with a NEW deadline (the human re-decision).
	caseB, workItemB := w.acceptedCase(t, "evt-handoff-store-2", "sess-handoff-store-2", `{"call":"inbound-call","from":"+79000000223"}`)
	conversationB := w.conversationFirst(t, caseB)
	turnB := w.modeTurnEvent(t, caseB, "sess-handoff-store-2", "evt-handoff-store-2-turn", `{"say":"позовите менеджера"}`)
	if err := store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "request_human", Key: "handoff-b-request-1",
		RequestHash: strings.Repeat("e", 64), Actor: w.ownershipActor().ID,
		CaseID: caseB, ConversationID: conversationB, WorkItemID: workItemB,
		TurnRef: turnB, ExpectedModeVersion: 1,
		HandoffTargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, HandoffAckDeadline: casesDomain.HandoffAck15m,
		ModeTermID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(), HandoffOutboxEventID: nextHandoffProbeID(),
		CorrelationID: nextHandoffProbeID(), Outcome: []byte(`{}`), Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the second case request must land, got %v", err)
	}
	assignB := casesPort.WorkCommandParams{
		Scope: scope, Command: "assign_human_handoff", Key: "handoff-b-assign-1",
		RequestHash: strings.Repeat("f", 64), Actor: w.ownershipActor().ID,
		CaseID: caseB, ConversationID: conversationB, WorkItemID: workItemB,
		HandoffAssignee: w.backup, HandoffBackup: w.owner.ID,
		HandoffAssigneeRole: casesDomain.HandoffTargetRoleOperator, HandoffAckDeadline: casesDomain.HandoffAck15m,
		OutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
		Outcome: []byte(`{}`), Now: ownershipNow,
	}
	if err := store.ExecuteWorkCommand(ctx, assignB); err != nil {
		t.Fatalf("the second case assignment must land, got %v", err)
	}
	// The non-assignee decline refuses FIRST (only the round's assignee
	// — the probe runs on the OPEN assigned round before the honest
	// refusal closes it).
	declineForeign := casesPort.WorkCommandParams{
		Scope: acknowledgeScope, Command: "decline_human_handoff", Key: "handoff-b-decline-foreign",
		RequestHash: strings.Repeat("d", 64), Actor: w.owner.ID,
		CaseID: caseB, ConversationID: conversationB, WorkItemID: workItemB,
		OutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
		Outcome: []byte(`{}`), Now: ownershipNow,
	}
	assertRuleError(t, store.ExecuteWorkCommand(ctx, declineForeign), "может только назначенный специалист")
	// The decline: the assignee refuses the round; the owner NEVER
	// changes (AC-6 — the prior safe owner stays); the request stays open.
	declineB := casesPort.WorkCommandParams{
		Scope: acknowledgeScope, Command: "decline_human_handoff", Key: "handoff-b-decline-1",
		RequestHash: strings.Repeat("d", 64), Actor: w.backup,
		CaseID: caseB, ConversationID: conversationB, WorkItemID: workItemB,
		OutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
		Outcome: []byte(`{}`), Now: ownershipNow,
	}
	if err := store.ExecuteWorkCommand(ctx, declineB); err != nil {
		t.Fatalf("the honest decline must land, got %v", err)
	}
	materialB, err := store.ReadHandoffMaterial(ctx, scope, caseB)
	if err != nil {
		t.Fatal(err)
	}
	viewB := casesDomain.DeriveHandoffState(materialB.Requests[0], materialB.Events, ownershipNow)
	if viewB.State != casesDomain.HandoffStateDeclined || !viewB.Active() {
		t.Fatalf("the declined round stays open for the re-assignment, got %+v", viewB)
	}
	if ownership := w.ownershipState(t, caseB); ownership.State.Current.Epoch != 1 ||
		ownership.State.Current.OwnerRef != w.owner.ID {
		t.Fatalf("the decline never changes the owner, got %+v", ownership.State.Current)
	}
	// The re-assignment: a NEW assigned event with a NEW deadline word
	// (the fresh envelope ids — every command carries its own).
	reassignB := assignB
	reassignB.Key = "handoff-b-assign-2"
	reassignB.HandoffAckDeadline = casesDomain.HandoffAck1h
	reassignB.OutboxEventID = nextHandoffProbeID()
	reassignB.CorrelationID = nextHandoffProbeID()
	if err := store.ExecuteWorkCommand(ctx, reassignB); err != nil {
		t.Fatalf("the re-assignment must land, got %v", err)
	}
	materialB, err = store.ReadHandoffMaterial(ctx, scope, caseB)
	if err != nil {
		t.Fatal(err)
	}
	viewB = casesDomain.DeriveHandoffState(materialB.Requests[0], materialB.Events, ownershipNow)
	if viewB.State != casesDomain.HandoffStateAssigned || len(materialB.Events) != 3 ||
		!viewB.DeadlineAt.Equal(ownershipNow.Add(time.Hour)) {
		t.Fatalf("the re-assignment is the new round with the new deadline, got %+v (%d events)", viewB, len(materialB.Events))
	}

	// The overdue derivation (the read-time truth — never a stored
	// status): the assigned round whose deadline passed without a closing
	// event derives overdue.
	lateView := casesDomain.DeriveHandoffState(materialB.Requests[0], materialB.Events, ownershipNow.Add(61*time.Minute))
	if lateView.State != casesDomain.HandoffStateOverdue || !lateView.Active() {
		t.Fatalf("the expired round derives overdue, got %+v", lateView)
	}
	// The overdue round refuses the acknowledge honestly (the deadline
	// closed it — the honest paths are the re-assignment, reassign or
	// take).
	overdueAck := casesPort.WorkCommandParams{
		Scope: acknowledgeScope, Command: "acknowledge_human_handoff", Key: "handoff-b-ack-overdue",
		RequestHash: strings.Repeat("a", 64), Actor: w.backup,
		CaseID: caseB, ConversationID: conversationB, WorkItemID: workItemB,
		ExpectedVersion:   1,
		TransferRequestID: nextHandoffProbeID(), TransferResolutionID: nextHandoffProbeID(),
		VerificationID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(),
		HandoffOutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
		Outcome: []byte(`{}`), Now: ownershipNow.Add(61 * time.Minute),
	}
	assertRuleError(t, store.ExecuteWorkCommand(ctx, overdueAck), "дедлайн подтверждения истёк")

	// The cancellation (a fresh case): the client's cancelling turn
	// anchors the event; the owner never changes; the history is never
	// deleted; the mode stays (the ratchet); the closed lifecycle opens
	// the door for a NEW ask (a fresh request number).
	caseC, workItemC := w.acceptedCase(t, "evt-handoff-store-3", "sess-handoff-store-3", `{"call":"inbound-call","from":"+79000000224"}`)
	conversationC := w.conversationFirst(t, caseC)
	turnC := w.modeTurnEvent(t, caseC, "sess-handoff-store-3", "evt-handoff-store-3-turn", `{"say":"позовите оператора"}`)
	if err := store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "request_human", Key: "handoff-c-request-1",
		RequestHash: strings.Repeat("e", 64), Actor: w.ownershipActor().ID,
		CaseID: caseC, ConversationID: conversationC, WorkItemID: workItemC,
		TurnRef: turnC, ExpectedModeVersion: 1,
		HandoffTargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, HandoffAckDeadline: casesDomain.HandoffAck15m,
		ModeTermID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(), HandoffOutboxEventID: nextHandoffProbeID(),
		CorrelationID: nextHandoffProbeID(), Outcome: []byte(`{}`), Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the third case request must land, got %v", err)
	}
	// The cancellation with a foreign turn anchor refuses; the honest one
	// lands.
	cancelParams := casesPort.WorkCommandParams{
		Scope: scope, Command: "cancel_human_request", Key: "handoff-c-cancel-1",
		RequestHash: strings.Repeat("c", 64), Actor: w.ownershipActor().ID,
		CaseID: caseC, ConversationID: conversationC, WorkItemID: workItemC,
		TurnRef:       nextHandoffProbeID(),
		OutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
		Outcome: []byte(`{}`), Now: ownershipNow,
	}
	assertRuleError(t, store.ExecuteWorkCommand(ctx, cancelParams), "turn_ref не найдено")
	cancellingTurn := w.modeTurnEvent(t, caseC, "sess-handoff-store-3", "evt-handoff-store-3-cancel", `{"say":"не нужно никого"}`)
	cancelParams.TurnRef = cancellingTurn
	if err := store.ExecuteWorkCommand(ctx, cancelParams); err != nil {
		t.Fatalf("the honest cancellation must land, got %v", err)
	}
	materialC, err := store.ReadHandoffMaterial(ctx, scope, caseC)
	if err != nil {
		t.Fatal(err)
	}
	viewC := casesDomain.DeriveHandoffState(materialC.Requests[0], materialC.Events, ownershipNow)
	if viewC.State != casesDomain.HandoffStateCancelled || viewC.Active() {
		t.Fatalf("the cancelled lifecycle is closed, got %+v", viewC)
	}
	if viewC.LastEvent == nil || viewC.LastEvent.TurnRef != cancellingTurn {
		t.Fatalf("the cancellation anchors the client's cancelling turn, got %+v", viewC.LastEvent)
	}
	if ownership := w.ownershipState(t, caseC); ownership.State.Current.Epoch != 1 {
		t.Fatalf("the cancellation never changes the owner, got %+v", ownership.State.Current)
	}
	if modeView := w.modeView(t, caseC); modeView.Active.Mode != casesDomain.ModeHumanRequested {
		t.Fatalf("the cancellation never lowers the mode (the ratchet), got %s", modeView.Active.Mode)
	}
	// The closed lifecycle opens the door for the NEW ask (the fresh
	// request number — the history is append-only).
	if err := store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "request_human", Key: "handoff-c-request-2",
		RequestHash: strings.Repeat("e", 64), Actor: w.ownershipActor().ID,
		CaseID: caseC, ConversationID: conversationC, WorkItemID: workItemC,
		TurnRef: turnC, ExpectedModeVersion: 2,
		HandoffTargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, HandoffAckDeadline: casesDomain.HandoffAck15m,
		ModeTermID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(), HandoffOutboxEventID: nextHandoffProbeID(),
		CorrelationID: nextHandoffProbeID(), Outcome: []byte(`{}`), Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the new ask after the closed lifecycle must land, got %v", err)
	}
	materialC, err = store.ReadHandoffMaterial(ctx, scope, caseC)
	if err != nil {
		t.Fatal(err)
	}
	if len(materialC.Requests) != 2 || materialC.Requests[1].RequestNo != 2 {
		t.Fatalf("the new ask is the request number 2 (the append-only history), got %+v", materialC.Requests)
	}
	// The second cancellation closes the second ask; then the same key
	// with ANOTHER material refuses with the honest command key conflict
	// (the closed lifecycle passes the competing-task gate — the store
	// command_records backstop fires).
	secondCancellingTurn := w.modeTurnEvent(t, caseC, "sess-handoff-store-3", "evt-handoff-store-3-cancel-2", `{"say":"всё, отменили"}`)
	if err := store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "cancel_human_request", Key: "handoff-c-cancel-2",
		RequestHash: strings.Repeat("c", 64), Actor: w.ownershipActor().ID,
		CaseID: caseC, ConversationID: conversationC, WorkItemID: workItemC,
		TurnRef:       secondCancellingTurn,
		OutboxEventID: nextHandoffProbeID(), CorrelationID: nextHandoffProbeID(),
		Outcome: []byte(`{}`), Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the second cancellation must land, got %v", err)
	}
	conflict := store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "request_human", Key: "handoff-c-request-2",
		RequestHash: strings.Repeat("e", 64), Actor: w.ownershipActor().ID,
		CaseID: caseC, ConversationID: conversationC, WorkItemID: workItemC,
		TurnRef: turnC, ExpectedModeVersion: 2,
		HandoffTargetRoles: []string{casesDomain.HandoffTargetRoleSupervision}, HandoffAckDeadline: casesDomain.HandoffAck1h,
		ModeTermID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(), HandoffOutboxEventID: nextHandoffProbeID(),
		CorrelationID: nextHandoffProbeID(), Outcome: []byte(`{}`), Now: ownershipNow,
	})
	if !errors.Is(conflict, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material must refuse with the command key conflict, got %v", conflict)
	}
}

// TestHandoffStoreFreezeGateHalves covers both halves of the Story 2.13
// freeze gate of the new commitments: the store half (the LIVE handoff
// derivation under a STRICTER mode — the app mode half passes under
// safety, the 00013 rows freeze the creation) and the control branch (a
// case without a mode and without a handoff keeps the 2.9 path
// byte-identical — the honest commitment lands).
func TestHandoffStoreFreezeGateHalves(t *testing.T) {
	w := newCasesWorld(t, 984)
	ctx := context.Background()
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	// The freeze gate store half (the handoff half under a STRICTER mode):
	// the safety ladder leaves the mode untouched (the ratchet), the ask
	// still lands (AC-1 — the ask is unconditional), and the new
	// commitment refuses in the STORE gate (the app mode half passes —
	// safety is not the frozen word; the live derivation of the 00013
	// rows freezes the creation).
	caseD, workItemD := w.acceptedCase(t, "evt-handoff-store-4", "sess-handoff-store-4", `{"call":"inbound-call","from":"+79000000225"}`)
	conversationD := w.conversationFirst(t, caseD)
	turnD := w.modeTurnEvent(t, caseD, "sess-handoff-store-4", "evt-handoff-store-4-turn", `{"say":"опасность"}`)
	w.proposeMode(t, casesAPI.ProposeModeCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseD, ConversationID: conversationD,
		Mode: casesDomain.ModeSafety, Scope: casesDomain.ModeScopeConversation, ExpectedModeVersion: 1,
		Reason: "сигнал безопасности в реплике", SignalSource: casesDomain.ModeSignalTurnEvent, TurnRef: turnD,
		IdempotencyKey: "handoff-safety-propose", Now: ownershipNow,
	})
	if err := store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "request_human", Key: "handoff-d-request-1",
		RequestHash: strings.Repeat("e", 64), Actor: w.ownershipActor().ID,
		CaseID: caseD, ConversationID: conversationD, WorkItemID: workItemD,
		TurnRef: turnD, ExpectedModeVersion: 2,
		HandoffTargetRoles: []string{casesDomain.HandoffTargetRoleSupervision}, HandoffAckDeadline: casesDomain.HandoffAck4h,
		ModeTermID: nextHandoffProbeID(), OutboxEventID: nextHandoffProbeID(), HandoffOutboxEventID: nextHandoffProbeID(),
		CorrelationID: nextHandoffProbeID(), Outcome: []byte(`{}`), Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the unconditional ask under the stricter mode must land, got %v", err)
	}
	// The mode stays safety (the ratchet — the ask never lowers it).
	if modeView := w.modeView(t, caseD); modeView.Active.Mode != casesDomain.ModeSafety {
		t.Fatalf("the stricter mode is never touched by the ask, got %s", modeView.Active.Mode)
	}
	// The new commitment refuses: the store half of the freeze gate (the
	// LIVE handoff derivation — the app mode half passes under safety).
	_, err = w.commitmentCreate(t, w.commitmentCreateCommand(caseD, workItemD, nextHandoffProbeID(),
		casesDomain.CommitmentStatusDraft, "handoff-freeze-store-half"))
	assertRuleError(t, err, "заморожено")
	// The control branch: a case without a mode and without a handoff
	// keeps the 2.9 path byte-identical (the honest commitment lands).
	caseE, workItemE := w.acceptedCase(t, "evt-handoff-store-5", "sess-handoff-store-5", `{"call":"inbound-call","from":"+79000000226"}`)
	if _, err := w.commitmentCreate(t, w.commitmentCreateCommand(caseE, workItemE, nextHandoffProbeID(),
		casesDomain.CommitmentStatusDraft, "handoff-control-commitment")); err != nil {
		t.Fatalf("the control branch commitment must land, got %v", err)
	}
}
