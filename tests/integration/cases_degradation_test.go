//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	casesStore "github.com/artemiosu/staffy/internal/cases/store"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5/pgconn"
)

// degradationSeedSeq keeps every seeded degradation-surface row globally
// unique across the whole suite (the accommodation precedent).
var degradationSeedSeq int64

// nextDegradationProbeID mints one globally unique probe id.
func nextDegradationProbeID() id.UUID {
	degradationSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 99700+degradationSeedSeq))
}

// The Story 2.12 step-02 probes: the migration 00012 invariants are
// pinned by the live probes immediately (the review R1 lesson of 2.7 —
// the invariant is never only a domain guard). Three probe families:
// the pg_class/RLS/grants discipline of the two append-only tables, the
// INSERT backstops (the cause/channel/kind dictionaries, the note bound,
// the revision and event number UNIQUEs, the v3 dictionaries of the
// policy outcomes, the update_promise word and the command/outbox
// ALTERs), and the store-level probes (the two record commands under the
// locks, the route guard, the monotonic revision and event number
// derivations, the append-only history, the read-time derivations, the
// honest outbox metadata and the perimeter absence in the fact
// channels).

// TestDegradationTablesGrantsAndRLS asserts the append-only discipline
// of the degradation and the delivery evidence surfaces at the database
// level: RLS + FORCE on both tables, INSERT/SELECT for staffy_app and no
// UPDATE, DELETE or TRUNCATE (the accommodation precedent of 2.11).
func TestDegradationTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('route_degradations', 'client_reply_delivery_events')`)
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
	for _, table := range []string{"route_degradations", "client_reply_delivery_events"} {
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

// degradationProbeRow carries the honest template of one degradation
// revision record with the per-probe dishonest knobs.
type degradationProbeRow struct {
	recordID   id.UUID
	connection id.UUID
	channel    string
	cause      string
	revision   int64
	cleared    bool
	note       string
}

// honestDegradationRow is the honest template of the record (the case's
// own route, the dictionary cause, the bounds).
func honestDegradationRow(connection id.UUID, recordID id.UUID) degradationProbeRow {
	return degradationProbeRow{
		recordID:   recordID,
		connection: connection,
		channel:    "phone",
		cause:      casesDomain.DegradationCauseTimeout,
		revision:   1,
		note:       "",
	}
}

// insertDegradationRow runs the direct INSERT of one degradation record
// row over the live case anchor (the admin role behind the RLS curtain —
// the DB-probe precedent of 2.7..2.11).
func (w *casesWorld) insertDegradationRow(t *testing.T, caseID id.UUID, row degradationProbeRow) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.route_degradations (tenant_id, case_id, record_id, connection_id, channel, cause, revision, cleared, note, actor_id, recorded_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		w.tenant, caseID, row.recordID, row.connection, row.channel, row.cause, row.revision, row.cleared, row.note, w.ownershipActor().ID, ownershipNow)
	return err
}

// insertEvidenceRow runs the direct INSERT of one delivery evidence
// event row over the seeded reply.
func (w *casesWorld) insertEvidenceRow(t *testing.T, caseID, replyID, eventID id.UUID, eventNo int64, kind string) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.client_reply_delivery_events (tenant_id, case_id, reply_id, event_id, event_no, kind, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
		w.tenant, caseID, replyID, eventID, eventNo, kind, w.ownershipActor().ID, ownershipNow)
	return err
}

// seedReplyForEvidence seeds one dispatched reply of the case (the
// direct INSERT of the reply row and its dispatched outcome — the
// evidence command target).
func (w *casesWorld) seedReplyForEvidence(t *testing.T, caseID, conversationID, replyID id.UUID, content string) {
	t.Helper()
	ctx := context.Background()
	if _, err := adminPool.Exec(ctx, `
                insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch,
                        writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version,
                        content, actor_id, created_at)
                values ($1,$2,$3,$4,'human',$5,1,'inbox',$6,'staffy-synthetic-telephony','phone','outbound',$7,1,1,$8,$9,$10)`,
		w.tenant, caseID, conversationID, replyID, w.ownershipActor().ID, w.connection, w.ownershipActor().ID, content, w.ownershipActor().ID, ownershipNow); err != nil {
		t.Fatalf("the seeded reply must land, got %v", err)
	}
	if _, err := adminPool.Exec(ctx, `
                insert into cases.client_reply_outcomes (tenant_id, outcome_id, reply_id, kind, created_at)
                values ($1,$2,$3,'dispatched',$4)`,
		w.tenant, nextDegradationProbeID(), replyID, ownershipNow); err != nil {
		t.Fatalf("the dispatched outcome must land, got %v", err)
	}
}

// TestDegradationDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00012 surface: the cause/channel/kind dictionaries,
// the note bound, the revision and event number monotonicity UNIQUEs,
// the reply FK of the evidence events, the v3 dictionaries of the policy
// outcomes (the degraded_mode class, the degraded_route reason, the
// rule_version v3 word, the permitted path words subset), the
// update_promise word of the replies and the command/outbox ALTERs —
// the honest shape is a database guarantee, not only a domain guard.
func TestDegradationDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 996)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-degradation-2-12-1", "sess-degradation-2-12-1", `{"call":"inbound-call","from":"+79000000212"}`)
	conversationID := w.conversationFirst(t, caseID)

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

	// The honest degradation rows: the case's own route (the world
	// connection, the phone channel), the dictionary cause, the bounds.
	if err := w.insertDegradationRow(t, caseID, honestDegradationRow(w.connection, nextDegradationProbeID())); err != nil {
		t.Fatalf("the honest record must land, got %v", err)
	}
	if err := w.insertDegradationRow(t, caseID, degradationProbeRow{recordID: nextDegradationProbeID(), connection: w.connection, channel: "phone", cause: casesDomain.DegradationCauseDisconnect, revision: 2}); err != nil {
		t.Fatalf("the revision 2 must land, got %v", err)
	}
	if err := w.insertDegradationRow(t, caseID, degradationProbeRow{recordID: nextDegradationProbeID(), connection: w.connection, channel: "phone", cause: casesDomain.DegradationCauseDisconnect, revision: 3, cleared: true}); err != nil {
		t.Fatalf("the clearing revision must land, got %v", err)
	}

	// The dictionary and bounds backstops of the degradation row.
	checkRefused("unknown channel", w.insertDegradationRow(t, caseID, degradationProbeRow{recordID: nextDegradationProbeID(), connection: w.connection, channel: "sms", cause: casesDomain.DegradationCauseTimeout, revision: 1}))
	checkRefused("unknown cause", w.insertDegradationRow(t, caseID, degradationProbeRow{recordID: nextDegradationProbeID(), connection: w.connection, channel: "phone", cause: "provider_napping", revision: 1}))
	checkRefused("note over the bound", w.insertDegradationRow(t, caseID, degradationProbeRow{recordID: nextDegradationProbeID(), connection: w.connection, channel: "phone", cause: casesDomain.DegradationCauseTimeout, revision: 1, note: strings.Repeat("н", 501)}))
	checkRefused("zero revision", w.insertDegradationRow(t, caseID, degradationProbeRow{recordID: nextDegradationProbeID(), connection: w.connection, channel: "phone", cause: casesDomain.DegradationCauseTimeout, revision: 0}))
	checkDuplicate("the raced revision", w.insertDegradationRow(t, caseID, degradationProbeRow{recordID: nextDegradationProbeID(), connection: w.connection, channel: "phone", cause: casesDomain.DegradationCauseRateLimited, revision: 1}))

	// The delivery evidence backstops: the seeded dispatched reply carries
	// the honest events; the dictionary and the monotonicity refuse the
	// dishonest forms.
	replyID := nextDegradationProbeID()
	w.seedReplyForEvidence(t, caseID, conversationID, replyID, "Канал сбоит — продолжим в течение часа")
	if err := w.insertEvidenceRow(t, caseID, replyID, nextDegradationProbeID(), 1, casesDomain.DeliveryEvidenceDeliveredConfirmed); err != nil {
		t.Fatalf("the honest evidence event must land, got %v", err)
	}
	if err := w.insertEvidenceRow(t, caseID, replyID, nextDegradationProbeID(), 2, casesDomain.DeliveryEvidenceDeliveryUnknown); err != nil {
		t.Fatalf("the second evidence event must land, got %v", err)
	}
	checkRefused("unknown evidence kind", w.insertEvidenceRow(t, caseID, replyID, nextDegradationProbeID(), 3, "maybe_delivered"))
	checkRefused("zero event number", w.insertEvidenceRow(t, caseID, replyID, nextDegradationProbeID(), 0, casesDomain.DeliveryEvidenceProviderFailed))
	checkDuplicate("the raced event number", w.insertEvidenceRow(t, caseID, replyID, nextDegradationProbeID(), 2, casesDomain.DeliveryEvidenceProviderFailed))
	// The FK backstop: the evidence of a foreign reply is impossible.
	_, err := adminPool.Exec(ctx, `
                insert into cases.client_reply_delivery_events (tenant_id, case_id, reply_id, event_id, event_no, kind, actor_id, created_at)
                values ($1,$2,$3,$4,1,$5,$6,$7)`,
		w.tenant, caseID, nextDegradationProbeID(), nextDegradationProbeID(), casesDomain.DeliveryEvidenceDeliveredConfirmed, w.ownershipActor().ID, ownershipNow)
	if err == nil {
		t.Fatalf("the foreign reply must be refused by the FK backstop")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.SQLState() != "23503" {
		t.Fatalf("the foreign reply refusal is the foreign_key_violation 23503, got %v", err)
	}

	// The v3 dictionaries of the policy outcomes: the degraded_mode class,
	// the degraded_route reason, the v3 rule word and the permitted path
	// words subset (the honest five words land, the foreign word and the
	// oversized array refuse). The FK chain needs the fresh proposal row
	// seeded on the case's first accepted event (the accommodation probe
	// precedent).
	insertOutcome := func(reason, class, rule, paths string) error {
		var turnRef id.UUID
		if err := adminPool.QueryRow(ctx, `
                        select event_id from cases.channel_events
                        where tenant_id=$1 and conversation_id=$2 and state='accepted'
                        order by received_at limit 1`, w.tenant, conversationID).Scan(&turnRef); err != nil {
			return err
		}
		proposalID := nextDegradationProbeID()
		if _, seedErr := adminPool.Exec(ctx, `
                        insert into cases.ai_proposals (tenant_id, case_id, conversation_id, work_item_id, proposal_id, turn_ref,
                                schema_version, intent, candidate_answer, missing_facts, extracted_facts, proposer, owner_epoch, mode_version, actor_id, created_at)
                        values ($1,$2,$3,$4,$5,$6,'ai-proposal-v1','price_inquiry','','[]'::jsonb,'[]'::jsonb,'synthetic_port',1,1,$7,$8)`,
			w.tenant, caseID, conversationID, workItemID, proposalID, turnRef, w.ownershipActor().ID, ownershipNow); seedErr != nil {
			return seedErr
		}
		_, execErr := adminPool.Exec(ctx, `
                        insert into cases.policy_outcomes (tenant_id, case_id, conversation_id, work_item_id, proposal_id, outcome_id,
                                outcome_no, outcome_class, reason, rule_version, intent, degraded_permitted_paths, input_snapshot, decision_digest, actor_id, created_at)
                        values ($1,$2,$3,$4,$5,$6,1,$7,$8,$9,'price_inquiry',$10::jsonb,'{}'::jsonb,repeat('a',64),$11,$12)`,
			w.tenant, caseID, conversationID, workItemID, proposalID, nextDegradationProbeID(), class, reason, rule, paths, w.ownershipActor().ID, ownershipNow)
		return execErr
	}
	allPaths := `["accept_info_no_promise","verified_channel_switch","human_handoff","agree_callback","stop_path"]`
	if err := insertOutcome(casesDomain.ReasonDegradedRoute, casesDomain.OutcomeDegradedMode, "intake-policy-v3", allPaths); err != nil {
		t.Fatalf("the v3 degraded outcome must land, got %v", err)
	}
	checkRefused("the foreign class word", insertOutcome(casesDomain.ReasonDegradedRoute, "degraded_commercial", "intake-policy-v3", allPaths))
	checkRefused("the foreign reason word", insertOutcome("route_sadness", casesDomain.OutcomeDegradedMode, "intake-policy-v3", allPaths))
	checkRefused("the foreign rule word", insertOutcome(casesDomain.ReasonDegradedRoute, casesDomain.OutcomeDegradedMode, "intake-policy-v4", allPaths))
	checkRefused("the foreign path word", insertOutcome(casesDomain.ReasonDegradedRoute, casesDomain.OutcomeDegradedMode, "intake-policy-v3", `["accept_info_no_promise","promise_a_price"]`))
	checkRefused("the oversized path array", insertOutcome(casesDomain.ReasonDegradedRoute, casesDomain.OutcomeDegradedMode, "intake-policy-v3", `["accept_info_no_promise","verified_channel_switch","human_handoff","agree_callback","stop_path","extra_path"]`))
	checkRefused("the non-array paths", insertOutcome(casesDomain.ReasonDegradedRoute, casesDomain.OutcomeDegradedMode, "intake-policy-v3", `"stop_path"`))

	// The update_promise word of the replies: the honest dictionary word
	// lands (the nullable word of the approved policy), the foreign word
	// refuses.
	if _, err := adminPool.Exec(ctx, `
                insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch,
                        writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version,
                        content, update_promise, actor_id, created_at)
                values ($1,$2,$3,$4,'human',$5,1,'inbox',$6,'staffy-synthetic-telephony','phone','outbound',$7,1,1,'Продолжим в течение часа',$8,$9,$10)`,
		w.tenant, caseID, conversationID, nextDegradationProbeID(), w.ownershipActor().ID, w.connection, w.ownershipActor().ID, casesDomain.UpdatePromiseWithin1h, w.ownershipActor().ID, ownershipNow); err != nil {
		t.Fatalf("the promised reply must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch,
                        writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version,
                        content, update_promise, actor_id, created_at)
                values ($1,$2,$3,$4,'human',$5,1,'inbox',$6,'staffy-synthetic-telephony','phone','outbound',$7,1,1,'Продолжим',$8,$9,$10)`,
		w.tenant, caseID, conversationID, nextDegradationProbeID(), w.ownershipActor().ID, w.connection, w.ownershipActor().ID, "within_2h", w.ownershipActor().ID, ownershipNow)
	checkRefused("the foreign promise word", err)

	// The command and outbox dictionary ALTERs: the two command words and
	// the two event types are the legal dictionary words (the dishonest
	// forms are refused).
	for _, word := range []string{"record_route_degradation", "record_delivery_evidence"} {
		if _, err := adminPool.Exec(ctx, `
                        insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                        values ($1,$2,$3,$4,repeat('b',64),'{}'::jsonb,$5)`,
			w.tenant, w.ownershipActor().ID, word, fmt.Sprintf("degradation-dict-probe-%d", degradationSeedSeq), ownershipNow); err != nil {
			t.Fatalf("the %s command word must land, got %v", word, err)
		}
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'note_route_failure',$3,repeat('b',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("degradation-dict-probe-%d", degradationSeedSeq), ownershipNow)
	checkRefused("foreign command word", err)
	for _, word := range []string{"case.route_degradation_recorded", "case.client_reply_delivery_evidence"} {
		if _, err := adminPool.Exec(ctx, `
                        insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                        values ($1,$2,$3,1,$4,$5,$6,$7,'{}'::jsonb,$8,$9)`,
			w.tenant, nextDegradationProbeID(), word, caseID, nextDegradationProbeID(), nextDegradationProbeID(), ownershipNow, fmt.Sprintf("degradation-outbox-probe-%d", degradationSeedSeq), ownershipNow); err != nil {
			t.Fatalf("the %s event word must land, got %v", word, err)
		}
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.route_sadness_recorded',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextDegradationProbeID(), caseID, nextDegradationProbeID(), nextDegradationProbeID(), ownershipNow, fmt.Sprintf("degradation-outbox-probe-%d", degradationSeedSeq), ownershipNow)
	checkRefused("foreign outbox event word", err)
}

// TestDegradationStoreRecordReads covers the store-level surface: the
// record command under the locks (the honest revision 1 with the outbox
// metadata), the monotonic revision derivation of the NEXT record, the
// clearing revision, the append-only history, the read-time derivation
// of the CURRENT slots, the route guard (the case's own route only),
// the domain guard refusals under the locks, the raced repeat (the
// stale signal), the key conflict of the same key with another
// material, the honest outbox envelope (the note never enters), the
// delivery evidence command (the monotonic event numbers, the reply
// guard, the honest envelope without the content) and the perimeter
// absence (no fact rows are ever written).
func TestDegradationStoreRecordReads(t *testing.T) {
	w := newCasesWorld(t, 997)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-degradation-store-1", "sess-degradation-store-1", `{"call":"inbound-call","from":"+79000000213"}`)
	conversationID := w.conversationFirst(t, caseID)
	store := casesStore.NewPostgres(runtimePool)
	// The command-probe scope: the ACTOR is the principal (the
	// command_records RLS policy demands actor == principal).
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	degradationParams := func(recordID, connection id.UUID, channel, cause, commandKey string, cleared bool, note string) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: scope, Command: "record_route_degradation", Key: commandKey,
			RequestHash: strings.Repeat("a", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID,
			DegradationRecordID: recordID, DegradationConnectionID: connection,
			DegradationChannel: channel, DegradationCause: cause,
			DegradationCleared: cleared, DegradationNote: note,
			CorrelationID: nextDegradationProbeID(), OutboxEventID: nextDegradationProbeID(),
			Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}

	// The honest revision 1: the timeout record of the case's own route
	// lands with the outbox event.
	firstRecord := nextDegradationProbeID()
	if err := store.ExecuteWorkCommand(ctx, degradationParams(firstRecord, w.connection, "phone", casesDomain.DegradationCauseTimeout, "degradation-record-1", false, "коннектор молчит")); err != nil {
		t.Fatalf("the honest degradation must land, got %v", err)
	}
	// The repeat of the same route: the revision 2 (the append-only
	// history — the store derives the next monotonic number under the
	// locks).
	secondRecord := nextDegradationProbeID()
	if err := store.ExecuteWorkCommand(ctx, degradationParams(secondRecord, w.connection, "phone", casesDomain.DegradationCauseDisconnect, "degradation-record-2", false, "")); err != nil {
		t.Fatalf("the revision 2 must land, got %v", err)
	}
	// The clearing revision: the route leaves the degraded set.
	if err := store.ExecuteWorkCommand(ctx, degradationParams(nextDegradationProbeID(), w.connection, "phone", casesDomain.DegradationCauseDisconnect, "degradation-record-3", true, "коннектор ожил")); err != nil {
		t.Fatalf("the clearing revision must land, got %v", err)
	}

	// The read: every revision travels ordered by the route and the
	// number; the CURRENT slots derive read-time (the cleared route is
	// restored, the history stays readable).
	records, err := store.ReadDegradations(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("expected the three revision records, got %d", len(records))
	}
	current := casesDomain.DeriveCurrentDegradations(records)
	route := casesDomain.RouteKey{ConnectionID: w.connection, Channel: "phone"}
	if current.RouteDegraded(route) {
		t.Fatalf("the cleared route is not degraded")
	}
	if state, known := current.RouteState(route); !known || state.Revision != 3 || !state.Cleared {
		t.Fatalf("the cleared state carries the latest revision, got %+v (known=%v)", state, known)
	}

	// The route guard: the route must exist in the conversations of the
	// case — a foreign connection and a foreign channel both refuse with
	// the honest rule word.
	foreignConnection := degradationParams(nextDegradationProbeID(), nextDegradationProbeID(), "phone", casesDomain.DegradationCauseTimeout, "degradation-route-foreign-connection", false, "")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, foreignConnection), "деградация чужого маршрута невозможна")
	foreignChannel := degradationParams(nextDegradationProbeID(), w.connection, "email", casesDomain.DegradationCauseTimeout, "degradation-route-foreign-channel", false, "")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, foreignChannel), "деградация чужого маршрута невозможна")

	// The guard probes under the locks (the honest 400 words of the domain
	// guard re-validated by the store).
	guardProbe := func(channel, cause, note string, fragment string) {
		t.Helper()
		err := store.ExecuteWorkCommand(ctx, degradationParams(nextDegradationProbeID(), w.connection, channel, cause, fmt.Sprintf("degradation-guard-%d", degradationSeedSeq), false, note))
		assertRuleError(t, err, fragment)
	}
	guardProbe("phone", "made_up_cause", "", "не входит в словарь")
	guardProbe("sms", casesDomain.DegradationCauseTimeout, "", "словарь каналов")
	guardProbe("phone", casesDomain.DegradationCauseTimeout, strings.Repeat("н", 501), "длиннее 500")

	// The raced repeat of the identical material (the same record id under
	// the same command key): the stale signal.
	raced := degradationParams(firstRecord, w.connection, "phone", casesDomain.DegradationCauseTimeout, "degradation-record-1", false, "коннектор молчит")
	if err := store.ExecuteWorkCommand(ctx, raced); !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("the raced identical record must refuse with the stale signal, got %v", err)
	}
	// The same key with ANOTHER material: the honest command key conflict.
	conflict := degradationParams(nextDegradationProbeID(), w.connection, "phone", casesDomain.DegradationCauseRateLimited, "degradation-record-1", false, "")
	if err := store.ExecuteWorkCommand(ctx, conflict); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material must refuse with the command key conflict, got %v", err)
	}
	// The unknown case: the honest 404 (the non-participant path).
	unknownCase := degradationParams(nextDegradationProbeID(), w.connection, "phone", casesDomain.DegradationCauseTimeout, "degradation-record-unknown", false, "")
	unknownCase.CaseID = nextDegradationProbeID()
	if err := store.ExecuteWorkCommand(ctx, unknownCase); !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the unknown case must refuse with the not-found word, got %v", err)
	}

	// The outbox honesty: the envelope of the honest record carries the
	// metadata only — the NOTE never enters (the operational material
	// stays in the row).
	var payload string
	if err := adminPool.QueryRow(ctx, `
                select payload from cases.outbox_events
                where tenant_id=$1 and dedup_key=$2`, w.tenant, casesDomain.DedupKeyRouteDegradationRecorded(secondRecord)).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "коннектор") {
		t.Fatalf("the outbox envelope never carries the note, got %s", payload)
	}
	var envelope casesDomain.RouteDegradationRecordedPayload
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ConnectionID != w.connection || envelope.Channel != "phone" ||
		envelope.Cause != casesDomain.DegradationCauseDisconnect || envelope.Revision != 2 || envelope.Cleared {
		t.Fatalf("the outbox envelope carries the route/cause/revision/cleared metadata, got %+v", envelope)
	}

	// The delivery evidence command: the seeded dispatched replies carry
	// the monotonic events; the honest kinds land.
	firstReply := nextDegradationProbeID()
	w.seedReplyForEvidence(t, caseID, conversationID, firstReply, "Канал сбоит — обновление в течение часа")
	secondReply := nextDegradationProbeID()
	w.seedReplyForEvidence(t, caseID, conversationID, secondReply, "Продолжим на другом канале")
	evidenceParams := func(eventID, replyID id.UUID, kind, commandKey string) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: scope, Command: "record_delivery_evidence", Key: commandKey,
			RequestHash: strings.Repeat("c", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID,
			DeliveryEvidenceEventID: eventID, DeliveryEvidenceReplyID: replyID,
			DeliveryEvidenceKind: kind,
			CorrelationID:        nextDegradationProbeID(), OutboxEventID: nextDegradationProbeID(),
			Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}
	firstEvent := nextDegradationProbeID()
	if err := store.ExecuteWorkCommand(ctx, evidenceParams(firstEvent, firstReply, casesDomain.DeliveryEvidenceDeliveryUnknown, "evidence-record-1")); err != nil {
		t.Fatalf("the honest evidence must land, got %v", err)
	}
	if err := store.ExecuteWorkCommand(ctx, evidenceParams(nextDegradationProbeID(), firstReply, casesDomain.DeliveryEvidenceDeliveredConfirmed, "evidence-record-2")); err != nil {
		t.Fatalf("the second evidence event must land, got %v", err)
	}
	// The read: the events travel ordered by the reply and the number; the
	// derivation of the first reply reads confirmed (the LAST event
	// decides), the second reply stays the honest unknown (dispatched
	// without events — AC-2).
	events, err := store.ReadDeliveryEvents(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].ReplyID != firstReply || events[0].EventNo != 1 || events[1].EventNo != 2 {
		t.Fatalf("expected the two ordered events of the first reply, got %+v", events)
	}
	state := casesDomain.DeriveReplyDeliveryState(true, eventsOfReply(events, firstReply))
	if state.State != casesDomain.ReplyDeliveryConfirmed || state.LastEventNo != 2 {
		t.Fatalf("the last event decides the confirmed state, got %+v", state)
	}
	state = casesDomain.DeriveReplyDeliveryState(true, eventsOfReply(events, secondReply))
	if state.State != casesDomain.ReplyDeliveryUnknown {
		t.Fatalf("dispatched without events is the honest unknown, got %+v", state)
	}
	state = casesDomain.DeriveReplyDeliveryState(false, eventsOfReply(events, firstReply))
	if state.State != casesDomain.ReplyDeliveryPrepared {
		t.Fatalf("without the dispatched outcome the truth of 2-4 stays prepared, got %+v", state)
	}

	// The evidence guards under the locks: the foreign kind word and the
	// foreign reply (and the mismatched conversation) refuse honestly.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, evidenceParams(nextDegradationProbeID(), firstReply, "maybe_delivered", fmt.Sprintf("evidence-guard-%d", degradationSeedSeq))), "не входит в словарь")
	if err := store.ExecuteWorkCommand(ctx, evidenceParams(nextDegradationProbeID(), nextDegradationProbeID(), casesDomain.DeliveryEvidenceProviderFailed, "evidence-foreign-reply")); !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the foreign reply must refuse with the not-found word, got %v", err)
	}
	wrongConversation := evidenceParams(nextDegradationProbeID(), firstReply, casesDomain.DeliveryEvidenceProviderFailed, "evidence-wrong-conversation")
	wrongConversation.ConversationID = nextDegradationProbeID()
	assertRuleError(t, store.ExecuteWorkCommand(ctx, wrongConversation), "не принадлежит этому обращению")

	// The raced repeat and the key conflict of the evidence command.
	racedEvidence := evidenceParams(firstEvent, firstReply, casesDomain.DeliveryEvidenceDeliveryUnknown, "evidence-record-1")
	if err := store.ExecuteWorkCommand(ctx, racedEvidence); !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("the raced identical evidence must refuse with the stale signal, got %v", err)
	}
	conflictEvidence := evidenceParams(nextDegradationProbeID(), firstReply, casesDomain.DeliveryEvidenceProviderFailed, "evidence-record-1")
	if err := store.ExecuteWorkCommand(ctx, conflictEvidence); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material must refuse with the command key conflict, got %v", err)
	}

	// The outbox honesty of the evidence: the envelope carries the reply
	// reference, the event number and the kind word only — the reply
	// CONTENT never enters.
	var evidencePayload string
	if err := adminPool.QueryRow(ctx, `
                select payload from cases.outbox_events
                where tenant_id=$1 and dedup_key=$2`, w.tenant, casesDomain.DedupKeyClientReplyDeliveryEvidence(firstEvent)).Scan(&evidencePayload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(evidencePayload, "обновление в течение часа") || strings.Contains(evidencePayload, "content") {
		t.Fatalf("the evidence envelope never carries the reply content, got %s", evidencePayload)
	}
	var evidenceEnvelope casesDomain.ClientReplyDeliveryEvidencePayload
	if err := json.Unmarshal([]byte(evidencePayload), &evidenceEnvelope); err != nil {
		t.Fatal(err)
	}
	if evidenceEnvelope.ReplyID != firstReply || evidenceEnvelope.EventNo != 1 || evidenceEnvelope.Kind != casesDomain.DeliveryEvidenceDeliveryUnknown {
		t.Fatalf("the evidence envelope carries the reply/event_no/kind metadata, got %+v", evidenceEnvelope)
	}

	// The perimeter probe (the degradations are NEVER facts): the records
	// never appear in the fact channels of the case (the live probe of the
	// 2.11 preferences precedent).
	var factRows int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.facts where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID).Scan(&factRows); err != nil {
		t.Fatal(err)
	}
	if factRows != 0 {
		t.Fatalf("the degradation records never write fact anchors, got %d rows", factRows)
	}
	var versionRows int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.fact_versions v
                join cases.facts f on f.tenant_id = v.tenant_id and f.fact_id = v.fact_id
                where f.tenant_id=$1 and f.case_id=$2`,
		w.tenant, caseID).Scan(&versionRows); err != nil {
		t.Fatal(err)
	}
	if versionRows != 0 {
		t.Fatalf("the degradation records never write fact versions, got %d rows", versionRows)
	}
}

// eventsOfReply filters the events of one reply (the derivation input
// helper of the probes).
func eventsOfReply(events []casesDomain.ReplyDeliveryEvent, replyID id.UUID) []casesDomain.ReplyDeliveryEvent {
	filtered := []casesDomain.ReplyDeliveryEvent{}
	for _, event := range events {
		if event.ReplyID == replyID {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
