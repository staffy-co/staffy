//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	casesStore "github.com/staffy-co/staffy/internal/cases/store"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
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

// TestDegradationMatrixRowsBySpec walks the 28 rows of the Story 2.12
// I/O & Edge-Case Matrix by name over the REAL service contour with PG:
// the honest record (the append-only row, the outbox envelope without the
// note, the panel derivation of the current slot), the independent second
// route, the dictionary guards, the append-only revisions with the
// clearing recovery, the replay and the key conflict, the scope gates
// BEFORE the material guards, the terminal boundary, the fact perimeter,
// the RLS/grants discipline, the degraded evaluation (the class, the
// reason, the five permitted paths, the question plan, the snapshot map,
// the deterministic digest), the control branches (the no-degradation
// decision identical to v2, the non-commercial informational path), the
// sales gate against the degraded outcome, the honest timing gate
// (required, foreign, the healthy control, the passing word), the
// delivery evidence states (unknown, confirmed, failed, the monotonic
// last-event history), the outbox perimeter, the transfer card material
// and the journeys pointer row.
func TestDegradationMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	w := newCasesWorld(t, 998)
	// The matrix world carries its own wide id sequence (the 2.10/2.11
	// precedent: the many-commanded rows drink the generator dry).
	matrixIDs := &sequenceIDs{values: make([]id.UUID, 512)}
	for i := range matrixIDs.values {
		matrixIDs.values[i] = id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 994000+i))
	}
	w.cases = w.buildCasesServiceClock(matrixIDs, fixedClock{ownershipNow})
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-degradation-matrix-1", "sess-degradation-matrix-1", `{"call":"inbound-call","from":"+79000000221"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-degradation-matrix-1", "evt-degradation-matrix-turn-1", `{"say":"сколько стоит деталь"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "matrix-degradation-sales")
	now := ownershipNow
	route := casesDomain.RouteKey{ConnectionID: w.connection, Channel: "phone"}
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	// The record shorthand of the rows (the honest template; the knobs
	// travel through the route, the cause, the note, the clearing mark and
	// the command key).
	record := func(connection id.UUID, channel, cause, note string, cleared bool, keyNo string, actor identityDomain.Principal, tenant id.UUID) (casesAPI.WorkOutcome, error) {
		return w.cases.RecordRouteDegradation(ctx, casesAPI.RecordRouteDegradationCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID,
			ConnectionID: connection, Channel: channel, Cause: cause, Cleared: cleared, Note: note,
			IdempotencyKey: keyNo, Now: now,
		})
	}
	// The view shorthand: the case view of the matrix case.
	viewOf := func() casesAPI.CaseView {
		detail, viewErr := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
		if viewErr != nil {
			t.Fatal(viewErr)
		}
		return detail
	}
	// The current outcome of one Work Item of one case (the maximal
	// number — the derivation input of the reply gates).
	currentOf := func(caseNo, item id.UUID) *casesDomain.PolicyOutcome {
		detail, viewErr := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseNo)
		if viewErr != nil {
			t.Fatal(viewErr)
		}
		for _, section := range detail.Policy.Items {
			if section.WorkItemID == item {
				return section.Current
			}
		}
		return nil
	}
	// The facts digest of the empty fact world — the perimeter row
	// compares it after every record (the degradations never enter the
	// digest).
	digestBefore := viewOf().Policy.FactsDigest

	// The control worlds of the policy rows: caseC walks the SAME
	// evaluation WITHOUT any degradation (the v2-identical control
	// branch); caseD walks the non-commercial intent on a degraded route
	// (the informational control branch); caseE serves the terminal
	// boundary row.
	caseC, workItemC := w.acceptedCase(t, "evt-degradation-matrix-2", "sess-degradation-matrix-2", `{"call":"inbound-call","from":"+79000000222"}`)
	conversationC := w.conversationFirst(t, caseC)
	turnC := w.modeTurnEvent(t, caseC, "sess-degradation-matrix-2", "evt-degradation-matrix-turn-2", `{"say":"сколько стоит деталь"}`)
	w.policySalesPrelude(t, caseC, conversationC, turnC, "matrix-degradation-control-sales")
	caseD, workItemD := w.acceptedCase(t, "evt-degradation-matrix-3", "sess-degradation-matrix-3", `{"call":"inbound-call","from":"+79000000223"}`)
	conversationD := w.conversationFirst(t, caseD)
	_ = w.modeTurnEvent(t, caseD, "sess-degradation-matrix-3", "evt-degradation-matrix-turn-3", `{"say":"у меня вопрос общего плана"}`)
	caseE, _ := w.acceptedCase(t, "evt-degradation-matrix-4", "sess-degradation-matrix-4", `{"call":"inbound-call","from":"+79000000224"}`)

	// Row 1 — the honest record of the dialog route (the timeout, the
	// revision 1): the append-only row pins the route/cause/revision and
	// the note; the outbox envelope carries the route words only (the
	// note never leaves the row); the panel section derives the current
	// slot with the honest cause text and the time (AC-1).
	t.Run("row-01 record: append-only row + outbox without note + panel current slot", func(t *testing.T) {
		first, err := record(w.connection, "phone", casesDomain.DegradationCauseTimeout, "коннектор молчит", false, "matrix-degradation-record-1", w.ownershipActor(), w.tenant)
		if err != nil || first.Command != "record_route_degradation" || first.DegradationRecordID == (id.UUID("")) || first.DegradationCause != casesDomain.DegradationCauseTimeout || first.DegradationCleared {
			t.Fatalf("the honest record must land with the command material, got %+v %v", first, err)
		}
		var revision int64
		var note, channel, cause string
		if err := adminPool.QueryRow(ctx, `
                        select channel, cause, revision, note from cases.route_degradations
                        where tenant_id=$1 and case_id=$2 and record_id=$3`,
			w.tenant, caseID, first.DegradationRecordID).Scan(&channel, &cause, &revision, &note); err != nil {
			t.Fatal(err)
		}
		if channel != "phone" || cause != casesDomain.DegradationCauseTimeout || revision != 1 || note != "коннектор молчит" {
			t.Fatalf("the append-only row pins the route/cause/revision/note, got %s/%s rev=%d note=%q", channel, cause, revision, note)
		}
		var payload string
		if err := adminPool.QueryRow(ctx, `
                        select payload from cases.outbox_events
                        where tenant_id=$1 and dedup_key=$2`, w.tenant, casesDomain.DedupKeyRouteDegradationRecorded(first.DegradationRecordID)).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, "коннектор молчит") || strings.Contains(payload, "note") {
			t.Fatalf("the outbox envelope never carries the note, got %s", payload)
		}
		var envelope casesDomain.RouteDegradationRecordedPayload
		if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.ConnectionID != w.connection || envelope.Channel != "phone" || envelope.Cause != casesDomain.DegradationCauseTimeout || envelope.Revision != 1 || envelope.Cleared {
			t.Fatalf("the envelope carries the route/cause/revision/cleared metadata, got %+v", envelope)
		}
		view := viewOf()
		if len(view.RouteDegradations.Current) != 1 {
			t.Fatalf("the panel derives the one current slot, got %+v", view.RouteDegradations.Current)
		}
		slot := view.RouteDegradations.Current[0]
		if slot.Channel != "phone" || slot.Cause != casesDomain.DegradationCauseTimeout || slot.Revision != 1 || slot.CauseText != casesDomain.DegradationCauseText(casesDomain.DegradationCauseTimeout) {
			t.Fatalf("the current slot carries the cause word with the honest text, got %+v", slot)
		}
		if slot.RecordedAt.IsZero() {
			t.Fatal("the current slot carries the recorded time")
		}
		if view.RouteDegradations.RecordsCount != 1 || len(view.RouteDegradations.History) != 1 || view.RouteDegradations.PanelText == "" {
			t.Fatalf("the panel section carries the honest history and the fixed words, got %+v", view.RouteDegradations)
		}
	})

	// Row 2 — the independent second route: the degradation of the first
	// route never touches the slot of another route (another connection
	// or another channel — «независимые маршруты не объявляются
	// недоступными без доказательства», AC-1); the derivation is per the
	// exact route key.
	t.Run("row-02 second route: the slot is never touched without its own row", func(t *testing.T) {
		view := viewOf()
		if len(view.RouteDegradations.Current) != 1 {
			t.Fatalf("only the degraded route carries the current slot, got %+v", view.RouteDegradations.Current)
		}
		records, err := store.ReadDegradations(ctx, scope, caseID)
		if err != nil {
			t.Fatal(err)
		}
		current := casesDomain.DeriveCurrentDegradations(records)
		otherChannel := casesDomain.RouteKey{ConnectionID: w.connection, Channel: "web_chat"}
		otherConnection := casesDomain.RouteKey{ConnectionID: nextDegradationProbeID(), Channel: "phone"}
		if current.RouteDegraded(otherChannel) || current.RouteDegraded(otherConnection) {
			t.Fatalf("the independent routes are never declared degraded without their own rows, got %+v", current)
		}
		if !current.RouteDegraded(route) {
			t.Fatal("the own route stays degraded (the control of the row)")
		}
	})

	// Row 3 — the delivery_unknown cause word: the operator relays the
	// ambiguous delivery observation of the connector — the dictionary
	// accepts it (AC-1 «неоднозначный delivery outcome»).
	t.Run("row-03 delivery_unknown cause: accepted by the dictionary", func(t *testing.T) {
		ambiguous, err := record(w.connection, "phone", casesDomain.DegradationCauseDeliveryUnknown, "квитанции нет", false, "matrix-degradation-record-2", w.ownershipActor(), w.tenant)
		if err != nil || ambiguous.DegradationCause != casesDomain.DegradationCauseDeliveryUnknown {
			t.Fatalf("the ambiguous delivery cause must land, got %+v %v", ambiguous, err)
		}
		var cause string
		var revision int64
		if err := adminPool.QueryRow(ctx, `
                        select cause, revision from cases.route_degradations
                        where tenant_id=$1 and case_id=$2 and record_id=$3`,
			w.tenant, caseID, ambiguous.DegradationRecordID).Scan(&cause, &revision); err != nil {
			t.Fatal(err)
		}
		if cause != casesDomain.DegradationCauseDeliveryUnknown || revision != 2 {
			t.Fatalf("the revision 2 carries the ambiguous cause, got %s rev=%d", cause, revision)
		}
	})

	// Row 4 — the guard dictionary of the record command: the unknown
	// cause, the channel outside the 2.1 dictionary, the note over the
	// rune bound and the route that does not exist in the conversations
	// of the case refuse with the honest 400 rule words BEFORE any SQL
	// (the domain truth); the route guard of the store refuses the
	// foreign route.
	t.Run("row-04 guards: cause/channel/bound/route 400 by domain + store guard", func(t *testing.T) {
		if _, err := record(w.connection, "phone", "provider_napping", "", false, "matrix-degradation-guard-cause", w.ownershipActor(), w.tenant); err == nil {
			t.Fatal("the unknown cause must refuse")
		} else {
			assertRuleError(t, err, "не входит в словарь")
		}
		if _, err := record(w.connection, "sms", casesDomain.DegradationCauseTimeout, "", false, "matrix-degradation-guard-channel", w.ownershipActor(), w.tenant); err == nil {
			t.Fatal("the foreign channel must refuse")
		} else {
			assertRuleError(t, err, "словарь каналов")
		}
		if _, err := record(w.connection, "phone", casesDomain.DegradationCauseTimeout, strings.Repeat("н", 501), false, "matrix-degradation-guard-note", w.ownershipActor(), w.tenant); err == nil {
			t.Fatal("the oversized note must refuse")
		} else {
			assertRuleError(t, err, "длиннее 500")
		}
		// The route guard: the foreign connection and the foreign channel
		// of the case's conversations both refuse (the store route guard
		// — a degradation of a foreign route is impossible).
		if _, err := record(nextDegradationProbeID(), "phone", casesDomain.DegradationCauseTimeout, "", false, "matrix-degradation-guard-route-1", w.ownershipActor(), w.tenant); err == nil {
			t.Fatal("the foreign connection route must refuse")
		} else {
			assertRuleError(t, err, "деградация чужого маршрута невозможна")
		}
		_, err = record(w.connection, "web_chat", casesDomain.DegradationCauseTimeout, "", false, "matrix-degradation-guard-route-2", w.ownershipActor(), w.tenant)
		assertRuleError(t, err, "деградация чужого маршрута невозможна")
	})

	// Row 5 — the repeat record of the same route with ANOTHER material
	// (another command key): the revision 3 lands append-only; the
	// history of the earlier revisions stays readable; the current slot
	// derives the revision 3.
	t.Run("row-05 repeat record: revision 3 append-only + history readable", func(t *testing.T) {
		third, err := record(w.connection, "phone", casesDomain.DegradationCauseDisconnect, "второй сбой", false, "matrix-degradation-record-3", w.ownershipActor(), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		var revision int64
		if err := adminPool.QueryRow(ctx, `
                        select revision from cases.route_degradations
                        where tenant_id=$1 and case_id=$2 and record_id=$3`,
			w.tenant, caseID, third.DegradationRecordID).Scan(&revision); err != nil {
			t.Fatal(err)
		}
		if revision != 3 {
			t.Fatalf("the repeat record lands the revision 3, got %d", revision)
		}
		view := viewOf()
		if view.RouteDegradations.RecordsCount != 3 || len(view.RouteDegradations.History) != 3 {
			t.Fatalf("the append-only history keeps every revision, got %d", view.RouteDegradations.RecordsCount)
		}
		if len(view.RouteDegradations.Current) != 1 || view.RouteDegradations.Current[0].Revision != 3 || view.RouteDegradations.Current[0].Cause != casesDomain.DegradationCauseDisconnect {
			t.Fatalf("the current slot derives the revision 3, got %+v", view.RouteDegradations.Current)
		}
	})

	// Row 6 — the clearing revision: the row lands honestly (its own
	// revision 4); the route leaves the current degraded set; the history
	// keeps the recovery row with the honest word.
	t.Run("row-06 cleared revision: route leaves the current set + honest history", func(t *testing.T) {
		if _, err := record(w.connection, "phone", casesDomain.DegradationCauseDisconnect, "коннектор ожил", true, "matrix-degradation-record-4", w.ownershipActor(), w.tenant); err != nil {
			t.Fatal(err)
		}
		view := viewOf()
		if len(view.RouteDegradations.Current) != 0 {
			t.Fatalf("the cleared route leaves the current set, got %+v", view.RouteDegradations.Current)
		}
		recoveryRow := false
		for _, row := range view.RouteDegradations.History {
			if row.Revision == 4 && row.Cleared && row.Note == "коннектор ожил" {
				recoveryRow = true
			}
		}
		if !recoveryRow || view.RouteDegradations.RecordsCount != 4 {
			t.Fatalf("the history keeps the recovery row honestly, got %+v", view.RouteDegradations.History)
		}
	})

	// Row 7 — the idempotent pair: the exact repeat of the row-1 command
	// replays the stored outcome (the append-only rows stay untouched);
	// the same key with another material is the honest command key
	// conflict.
	t.Run("row-07 replay + key conflict 409", func(t *testing.T) {
		replayed, err := record(w.connection, "phone", casesDomain.DegradationCauseTimeout, "коннектор молчит", false, "matrix-degradation-record-1", w.ownershipActor(), w.tenant)
		if err != nil || !replayed.Replay {
			t.Fatalf("the exact repeat replays the stored outcome, got %+v %v", replayed, err)
		}
		var revisions int64
		if err := adminPool.QueryRow(ctx, `
                        select count(*) from cases.route_degradations
                        where tenant_id=$1 and case_id=$2`,
			w.tenant, caseID).Scan(&revisions); err != nil {
			t.Fatal(err)
		}
		if revisions != 4 {
			t.Fatalf("the replay never appends a revision, got %d", revisions)
		}
		if _, err := record(w.connection, "phone", casesDomain.DegradationCauseTimeout, "другой материал той же команды", false, "matrix-degradation-record-1", w.ownershipActor(), w.tenant); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
			t.Fatalf("the same key with another material refuses with the conflict, got %v", err)
		}
	})

	// Row 8 — the rights gates BEFORE the material guards (the M1
	// precedent): the audit-only member reads the scope words (403) and
	// the dictionary violation never masks the scope refusal; the member
	// of another tenant reads the plain 404 (the RLS curtain).
	t.Run("row-08 gates: audit-only 403 BEFORE guards; foreign tenant 404", func(t *testing.T) {
		auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "matrix-degradation-audit-998@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000009981", "000000009982")
		auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
		if _, err := record(w.connection, "phone", casesDomain.DegradationCauseTimeout, "", false, "matrix-degradation-audit-1", auditOnly, w.tenant); !errors.Is(err, identityDomain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("the audit-only member must be forbidden before the guards, got %v", err)
		}
		if _, err := record(w.connection, "phone", "made_up_cause", "", false, "matrix-degradation-audit-2", auditOnly, w.tenant); !errors.Is(err, identityDomain.ErrForbidden) {
			t.Fatalf("the dictionary violation never masks the scope refusal, got %v", err)
		}
		foreignTenant := createForeignTenant(t, w)
		foreign := inviteMember(t, w.identity, w.owner, foreignTenant, "matrix-degradation-foreign-998@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Review}, "000000009983", "000000009984")
		if _, err := record(w.connection, "phone", casesDomain.DegradationCauseTimeout, "", false, "matrix-degradation-foreign-1", foreign, w.tenant); !errors.Is(err, casesDomain.ErrNotFound) && !errors.Is(err, identityDomain.ErrNotFound) {
			t.Fatalf("the member of another tenant must read the plain 404, got %v", err)
		}
	})

	// Row 9 — the terminal case boundary: the record commands refuse the
	// non-open case honestly (the app guard «обслуживать нечего»); the v1
	// state dictionary of the slice carries the open word ONLY — the
	// terminal state word cannot exist in this slice (the DB CHECK is the
	// honest boundary; the app guard stays for the future dictionary
	// growth — the 2.11 precedent).
	t.Run("row-09 terminal case: the state dictionary keeps the boundary honest", func(t *testing.T) {
		_, err := adminPool.Exec(ctx, `
                        update cases.cases set state='resolved' where tenant_id=$1 and case_id=$2`,
			w.tenant, caseE)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("the terminal state word is impossible in this slice (the CHECK refusal), got %v", err)
		}
		// The open case still records (the boundary never blocks the
		// honest path).
		if _, err := w.cases.RecordRouteDegradation(ctx, casesAPI.RecordRouteDegradationCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseE,
			ConnectionID: w.connection, Channel: "phone",
			Cause: casesDomain.DegradationCauseRateLimited, Cleared: false, Note: "открытый кейс пишет",
			IdempotencyKey: "matrix-degradation-record-open", Now: now,
		}); err != nil {
			t.Fatalf("the open case records honestly, got %v", err)
		}
	})

	// Row 10 — the fact perimeter: the degradation rows are NEVER facts —
	// no fact anchors, no fact versions and the facts digest of the case
	// never moved (the live probe of AC-1 «деградация не факт и не
	// доказательство»).
	t.Run("row-10 perimeter: never facts, the digest never moves", func(t *testing.T) {
		for _, probe := range []struct {
			query string
			word  string
		}{
			{`select count(*) from cases.facts where tenant_id=$1 and case_id=$2`, "no fact anchors"},
			{`select count(*) from cases.fact_versions where tenant_id=$1 and case_id=$2`, "no fact versions"},
			{`select count(*) from cases.verification_records where tenant_id=$1 and case_id=$2`, "no verification records"},
		} {
			var rows int
			if err := adminPool.QueryRow(ctx, probe.query, w.tenant, caseID).Scan(&rows); err != nil {
				t.Fatal(err)
			}
			if rows != 0 {
				t.Fatalf("%s: the degradation rows never become the fact material, got %d", probe.word, rows)
			}
		}
		if digest := viewOf().Policy.FactsDigest; digest != digestBefore {
			t.Fatalf("the facts digest never moves (the degradations are not the fact material), got %s after %s", digest, digestBefore)
		}
	})

	// Row 11 — the RLS/grants discipline of the migration 00012 (the live
	// probe of the probe family above; the row keeps the traceable core:
	// both append-only tables carry RLS + FORCE and never grant
	// UPDATE/DELETE/TRUNCATE).
	t.Run("row-11 RLS + INSERT/SELECT-only of both tables", func(t *testing.T) {
		for _, table := range []string{"cases.route_degradations", "cases.client_reply_delivery_events"} {
			var rls, force bool
			if err := runtimePool.QueryRow(ctx, `
                                select relrowsecurity, relforcerowsecurity from pg_class
                                where relnamespace='cases'::regnamespace and relname=$1`, strings.SplitN(table, ".", 2)[1]).Scan(&rls, &force); err != nil {
				t.Fatal(err)
			}
			if !rls || !force {
				t.Fatalf("%s must carry RLS + FORCE, got %v/%v", table, rls, force)
			}
			for _, grant := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
				var allowed bool
				if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, table, grant).Scan(&allowed); err != nil {
					t.Fatal(err)
				}
				if allowed {
					t.Fatalf("%s on %s must never be granted", grant, table)
				}
			}
		}
	})

	// The proposal/evaluation shorthands of the policy rows (the honest
	// anchors of the case view; the turn ref of the first accepted event).
	turnRefOf := func(caseNo id.UUID) id.UUID {
		detail, viewErr := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseNo)
		if viewErr != nil {
			t.Fatal(viewErr)
		}
		for _, event := range detail.Timeline {
			if event.State == casesDomain.EventStateAccepted {
				return event.EventID
			}
		}
		t.Fatal("the case carries no accepted turn for the proposal")
		return id.UUID("")
	}
	submitProposal := func(caseNo, conv, item id.UUID, intent, answer, keyNo string) casesAPI.WorkOutcome {
		proposal, propErr := w.cases.SubmitAIProposal(ctx, casesAPI.SubmitAIProposalCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseNo,
			ConversationID: conv, WorkItemID: item, TurnRef: turnRefOf(caseNo),
			Intent: intent, CandidateAnswer: answer,
			Proposer:       casesDomain.ProposerSyntheticPort,
			IdempotencyKey: keyNo, Now: now,
		})
		if propErr != nil {
			t.Fatal(propErr)
		}
		return proposal
	}
	evaluate := func(caseNo, item, proposalID id.UUID, keyNo string) (casesAPI.WorkOutcome, error) {
		detail, viewErr := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseNo)
		if viewErr != nil {
			t.Fatal(viewErr)
		}
		version := int64(0)
		for _, wi := range detail.WorkItems {
			if wi.WorkItemID == item {
				version = wi.Version
			}
		}
		return w.cases.EvaluatePolicy(ctx, casesAPI.EvaluatePolicyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseNo, ProposalID: proposalID,
			ExpectedModeVersion: detail.Policy.ModeVersion, ExpectedOwnerEpoch: detail.Policy.OwnerEpoch,
			ExpectedFactsDigest: detail.Policy.FactsDigest, ExpectedWorkItemVersion: version,
			IdempotencyKey: keyNo, Now: now,
		})
	}

	// Row 12 — the degraded evaluation of the commercial intent on the
	// degraded dialog route: the class degraded_mode, the reason
	// degraded_route, the five permitted path words of the pack, the
	// question plan STAYS (questions are not promises), the rule version
	// v3, the input snapshot carries the canonical route map and the
	// digest is deterministic (the re-evaluation of the same anchors —
	// the same digest, the grown number).
	t.Run("row-12 degraded evaluation: class + reason + 5 paths + questions + v3 + snapshot + digest", func(t *testing.T) {
		if _, err := record(w.connection, "phone", casesDomain.DegradationCauseTimeout, "канал снова молчит", false, "matrix-degradation-record-5", w.ownershipActor(), w.tenant); err != nil {
			t.Fatal(err)
		}
		proposal := submitProposal(caseID, conversationID, workItemID, casesDomain.IntentPriceInquiry, "Предлагаю деталь", "matrix-degradation-proposal-1")
		if _, err := evaluate(caseID, workItemID, proposal.ProposalID, "matrix-degradation-evaluate-1"); err != nil {
			t.Fatal(err)
		}
		outcome := currentOf(caseID, workItemID)
		if outcome == nil || outcome.OutcomeClass != casesDomain.OutcomeDegradedMode || outcome.Reason != casesDomain.ReasonDegradedRoute {
			t.Fatalf("the degraded route evaluation carries the degraded mode class, got %+v", outcome)
		}
		if len(outcome.DegradedPermittedPaths) != len(casesDomain.DegradedPermittedPathWords) {
			t.Fatalf("the outcome carries the full pack of the permitted paths, got %+v", outcome.DegradedPermittedPaths)
		}
		for _, word := range casesDomain.DegradedPermittedPathWords {
			if !inWords(outcome.DegradedPermittedPaths, word) {
				t.Fatalf("the permitted path %s must travel with the outcome, got %+v", word, outcome.DegradedPermittedPaths)
			}
		}
		serviceKind, urgency := false, false
		for _, fact := range outcome.MandatoryFacts {
			if fact.MaterialKey == "service_kind" && fact.State == casesDomain.MandatoryStateAsk {
				serviceKind = true
			}
			if fact.MaterialKey == "urgency" && fact.State == casesDomain.MandatoryStateAsk {
				urgency = true
			}
		}
		if !serviceKind || !urgency {
			t.Fatalf("the question plan stays (the questions are never promises), got %+v", outcome.MandatoryFacts)
		}
		for _, step := range outcome.NextSteps {
			if step.Step == casesDomain.StepAwaitConfirmation {
				t.Fatal("the degraded outcome never awaits the commercial confirmation")
			}
		}
		if outcome.RuleVersion != casesDomain.PolicyRuleVersion {
			t.Fatalf("the rule version is the honest current word, got %s (constant %s)", outcome.RuleVersion, casesDomain.PolicyRuleVersion)
		}
		state, known := outcome.InputSnapshot.Degradations[route.String()]
		if !known || state.Cause != casesDomain.DegradationCauseTimeout || state.Revision != 5 || state.Cleared {
			t.Fatalf("the input snapshot carries the canonical route map, got %+v", outcome.InputSnapshot.Degradations)
		}
		before := outcome
		if _, err := evaluate(caseID, workItemID, proposal.ProposalID, "matrix-degradation-reevaluate-1"); err != nil {
			t.Fatal(err)
		}
		after := currentOf(caseID, workItemID)
		if after.DecisionDigest != before.DecisionDigest || after.OutcomeNo <= before.OutcomeNo {
			t.Fatalf("the same inputs produce the same digest with the grown number, got %s/%d after %s/%d",
				after.DecisionDigest, after.OutcomeNo, before.DecisionDigest, before.OutcomeNo)
		}
	})

	// Row 13 — the control branch: the SAME evaluation WITHOUT the
	// degradations → the decision fields are the v2 shape byte-identically
	// (commercial_answer + ok_commercial + the questions + the await
	// confirmation, no permitted paths; the pre-2.12 families of 2.6 pin
	// the byte identity — the row pins the shape); the snapshot map is
	// the initialized empty map (the v3 digest of the same state); the
	// question plan is identical to the degraded world (the plan never
	// depends on the route state).
	t.Run("row-13 control branch: the decision without degradations stays the v2 shape", func(t *testing.T) {
		proposal := submitProposal(caseC, conversationC, workItemC, casesDomain.IntentPriceInquiry, "Предлагаю деталь", "matrix-degradation-control-proposal-1")
		if _, err := evaluate(caseC, workItemC, proposal.ProposalID, "matrix-degradation-control-evaluate-1"); err != nil {
			t.Fatal(err)
		}
		outcome := currentOf(caseC, workItemC)
		if outcome == nil || outcome.OutcomeClass != casesDomain.OutcomeCommercialAnswer || outcome.Reason != casesDomain.ReasonOkCommercial {
			t.Fatalf("the no-degradation control carries the v2 commercial decision, got %+v", outcome)
		}
		await := false
		for _, step := range outcome.NextSteps {
			if step.Step == casesDomain.StepAwaitConfirmation {
				await = true
			}
		}
		if !await || len(outcome.DegradedPermittedPaths) != 0 {
			t.Fatalf("the v2 decision shape: the confirmation step and no degraded paths, got %+v", outcome)
		}
		if len(outcome.InputSnapshot.Degradations) != 0 {
			t.Fatalf("the control snapshot carries the initialized empty map, got %+v", outcome.InputSnapshot.Degradations)
		}
		degraded := currentOf(caseID, workItemID)
		if len(outcome.MandatoryFacts) != len(degraded.MandatoryFacts) {
			t.Fatalf("the question plans are identical (the plan never depends on the route state), got %+v vs %+v", outcome.MandatoryFacts, degraded.MandatoryFacts)
		}
	})

	// Row 14 — the non-commercial control: the degradation of the route
	// with a NON-commercial intent → the informational answer as before
	// (the information is allowed on a degraded route — the «короткое
	// сообщение» of AC-4 is never a promise).
	t.Run("row-14 non-commercial intent: informational as before", func(t *testing.T) {
		if _, err := w.cases.RecordRouteDegradation(ctx, casesAPI.RecordRouteDegradationCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseD,
			ConnectionID: w.connection, Channel: "phone",
			Cause: casesDomain.DegradationCauseTimeout, Cleared: false, Note: "",
			IdempotencyKey: "matrix-degradation-info-record-1", Now: now,
		}); err != nil {
			t.Fatal(err)
		}
		proposal := submitProposal(caseD, conversationD, workItemD, casesDomain.IntentGeneralQuestion, "Отвечу общими словами", "matrix-degradation-info-proposal-1")
		if _, err := evaluate(caseD, workItemD, proposal.ProposalID, "matrix-degradation-info-evaluate-1"); err != nil {
			t.Fatal(err)
		}
		outcome := currentOf(caseD, workItemD)
		if outcome == nil || outcome.OutcomeClass != casesDomain.OutcomeInformationalAnswer || len(outcome.DegradedPermittedPaths) != 0 {
			t.Fatalf("the information is allowed on the degraded route, got %+v", outcome)
		}
	})

	// Row 15 — the fail-closed stop of the EMPTY pack dictionary: the
	// unit-level probe (the pack dictionary is compile-time — the
	// integration contour cannot empty it; the domain family pins the
	// honest stop).
	t.Run("row-15 fail-closed empty pack (the unit probe)", func(t *testing.T) {
		t.Skip("the domain unit test TestEvaluatePolicyDegradedEmptyPathsFailClosed walks this row (the pack dictionary is compile-time)")
	})

	// Row 16 — the sales gate against the degraded outcome: the acting
	// outcome of the Work Item is degraded_mode → the sales reply refuses
	// with the honest 2-6 gate (only commercial_answer opens the sales
	// path — the promises on the degraded route are closed machine-wise).
	t.Run("row-16 sales reply against degraded_mode: 400 of the policy gate 2-6", func(t *testing.T) {
		_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
			Content: "Сориентирую по цене", IdempotencyKey: "matrix-degradation-reply-sales", Now: now,
		})
		var policyGate casesDomain.ReplyPolicyError
		if !errors.As(err, &policyGate) || policyGate.PermittedPath != casesDomain.PermittedPathEvaluatePolicy {
			t.Fatalf("expected the policy gate refusal of the degraded outcome, got %v", err)
		}
		if !strings.Contains(policyGate.Reason, "commercial_answer") {
			t.Fatalf("the refusal names the only permitted class, got %s", policyGate.Reason)
		}
	})

	// Row 17 — the honest timing gate (classless): the reply on the
	// DEGRADED route without the update promise word refuses with the
	// honest required words carrying the dictionary.
	t.Run("row-17 reply without promise on degraded route: 400 required with the dictionary", func(t *testing.T) {
		_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			Content: "Канал сбоит", IdempotencyKey: "matrix-degradation-reply-required", Now: now,
		})
		assertRuleError(t, err, "обязано обещать время следующего обновления")
		var ruleErr casesDomain.WorkRuleError
		if errors.As(err, &ruleErr) && ruleErr.Rule != "update_promise_required" {
			t.Fatalf("the machine rule word is the required one, got %s", ruleErr.Rule)
		}
		if err != nil && !strings.Contains(err.Error(), casesDomain.UpdatePromiseWithin4h) {
			t.Fatalf("the required refusal names the dictionary words, got %v", err)
		}
	})

	// Row 18 — the promise word outside the approved dictionary: the
	// honest foreign (a word outside the policy never leaves the
	// machine).
	t.Run("row-18 foreign promise word: 400 foreign", func(t *testing.T) {
		_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			UpdatePromise: "maybe_tomorrow", Content: "Канал сбоит",
			IdempotencyKey: "matrix-degradation-reply-foreign", Now: now,
		})
		assertRuleError(t, err, "не входит в утверждённую policy")
		var ruleErr casesDomain.WorkRuleError
		if errors.As(err, &ruleErr) && ruleErr.Rule != "update_promise_foreign" {
			t.Fatalf("the machine rule word is the foreign one, got %s", ruleErr.Rule)
		}
	})

	// Row 19 — the healthy route control: the reply with a non-empty
	// promise on the HEALTHY route refuses with the foreign word (the
	// field has exactly one meaning); the empty word passes the old path
	// byte-identically (the control branch of the pre-2.12 worlds).
	t.Run("row-19 healthy route: foreign word 400; empty passes the old path", func(t *testing.T) {
		_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseC, ConversationID: conversationC,
			ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			UpdatePromise: casesDomain.UpdatePromiseWithin1h, Content: "Обычное сообщение",
			IdempotencyKey: "matrix-degradation-reply-control-1", Now: now,
		})
		assertRuleError(t, err, "маршрут не деградирован")
		control, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseC, ConversationID: conversationC,
			ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			Content:        "Обычное сообщение без обещания",
			IdempotencyKey: "matrix-degradation-reply-control-2", Now: now,
		})
		if err != nil || control.ReplyState != "prepared" || control.UpdatePromise != "" {
			t.Fatalf("the empty word passes the old path, got %+v %v", control, err)
		}
	})

	// Row 20 — the honest pass: the reply on the degraded route carries
	// the promise word of the approved dictionary — the full ladder
	// passes; the reply ROW carries the word; the panel shows the honest
	// text of the dictionary.
	t.Run("row-20 promised reply: passes the ladder + the row carries the word", func(t *testing.T) {
		passed, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			UpdatePromise: casesDomain.UpdatePromiseWithin4h, Content: "Канал сбоит — продолжим",
			IdempotencyKey: "matrix-degradation-reply-pass-1", Now: now,
		})
		if err != nil || passed.ReplyState != "prepared" || passed.UpdatePromise != casesDomain.UpdatePromiseWithin4h {
			t.Fatalf("the promised reply passes the full ladder carrying the word, got %+v %v", passed, err)
		}
		var promise *string
		if err := adminPool.QueryRow(ctx, `
                        select update_promise from cases.client_replies
                        where tenant_id=$1 and reply_id=$2`, w.tenant, passed.ReplyID).Scan(&promise); err != nil {
			t.Fatal(err)
		}
		if promise == nil || *promise != casesDomain.UpdatePromiseWithin4h {
			t.Fatalf("the reply row keeps the promise word, got %v", promise)
		}
		view := viewOf()
		found := false
		for _, card := range view.Ownership.Replies {
			if card.Reply.ReplyID == passed.ReplyID && card.Reply.UpdatePromise == casesDomain.UpdatePromiseWithin4h {
				found = true
			}
		}
		if !found {
			t.Fatalf("the panel reply row carries the word, got %+v", view.Ownership.Replies)
		}
	})

	// The delivery evidence rows need the DISPATCHED replies: the three
	// honest promise replies of the degraded route + the multi-event
	// probe reply land before the single drain (the 2-4 truth: dispatched
	// is the record of the record — the delivery state derivation reads
	// it).
	for _, probe := range []struct {
		keyNo   string
		promise string
	}{
		{"matrix-degradation-reply-pass-2", casesDomain.UpdatePromiseNextBusinessDay},
		{"matrix-degradation-reply-pass-3", casesDomain.UpdatePromiseNoPromise},
		{"matrix-degradation-reply-pass-4", casesDomain.UpdatePromiseWithin1h},
	} {
		if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			UpdatePromise: probe.promise, Content: "Канал сбоит — продолжим",
			IdempotencyKey: probe.keyNo, Now: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.cases.DrainClientReplies(ctx, w.ownershipActor(), w.tenant); err != nil {
		t.Fatal(err)
	}
	// The reply shorthand of the delivery rows: the reply id by the row
	// order of the case view (the newest replies travel last).
	replyOf := func(indexFromEnd int) id.UUID {
		view := viewOf()
		if len(view.Ownership.Replies) == 0 {
			t.Fatal("the case carries the replies")
		}
		return view.Ownership.Replies[len(view.Ownership.Replies)-1-indexFromEnd].Reply.ReplyID
	}
	// The delivery state of one reply of the case view.
	deliveryOf := func(replyID id.UUID) (casesAPI.ReplyDeliveryView, bool) {
		view := viewOf()
		for _, delivery := range view.Ownership.Deliveries {
			if delivery.ReplyID == replyID {
				return delivery, true
			}
		}
		return casesAPI.ReplyDeliveryView{}, false
	}
	// The evidence shorthand: the operator relays one connector
	// observation of one reply.
	evidence := func(replyID id.UUID, kind, keyNo string) (casesAPI.WorkOutcome, error) {
		return w.cases.RecordDeliveryEvidence(ctx, casesAPI.RecordDeliveryEvidenceCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
			ReplyID: replyID, Kind: kind,
			IdempotencyKey: keyNo, Now: now,
		})
	}

	// Row 21 — the honest unknown: the dispatched reply without any
	// evidence event is the UNKNOWN — «результат становится unknown, а не
	// delivered» (AC-2: dispatched is a record of a record, never a
	// delivery); the panel carries the state word with the time of the
	// outcome.
	t.Run("row-21 dispatched without evidence: the honest unknown, never delivered", func(t *testing.T) {
		for index := 0; index < 4; index++ {
			delivery, known := deliveryOf(replyOf(index))
			if !known {
				t.Fatalf("the delivery state of the reply must derive, got %+v", delivery)
			}
			if delivery.State != casesDomain.ReplyDeliveryUnknown {
				t.Fatalf("dispatched without events is the honest unknown, got %+v", delivery)
			}
		}
	})

	// Row 22 — the confirmed receipt: the evidence delivered_confirmed
	// derives the confirmed state; the duplicate is never sent (the drain
	// is single-record — the 2-4 UNIQUE discipline).
	t.Run("row-22 evidence delivered_confirmed: confirmed + no duplicate", func(t *testing.T) {
		confirmedReply := replyOf(3)
		outcome, err := evidence(confirmedReply, casesDomain.DeliveryEvidenceDeliveredConfirmed, "matrix-degradation-evidence-1")
		if err != nil || outcome.Command != "record_delivery_evidence" || outcome.DeliveryKindWord != casesDomain.DeliveryEvidenceDeliveredConfirmed {
			t.Fatalf("the confirmed evidence must land, got %+v %v", outcome, err)
		}
		delivery, known := deliveryOf(confirmedReply)
		if !known || delivery.State != casesDomain.ReplyDeliveryConfirmed || delivery.LastEventNo != 1 || delivery.LastKind != casesDomain.DeliveryEvidenceDeliveredConfirmed {
			t.Fatalf("the receipt observation derives the confirmed state with the timeline anchor, got %+v", delivery)
		}
		if delivery.LastEventAt.IsZero() {
			t.Fatal("the timeline carries the event time (AC-9)")
		}
		again, err := w.cases.DrainClientReplies(ctx, w.ownershipActor(), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if again.Dispatched != 0 {
			t.Fatalf("the duplicate is never sent (the single-record drain), got %+v", again)
		}
	})

	// Row 23 — the ambiguous receipt: the evidence delivery_unknown keeps
	// the unknown state; the retry decision stays with the declared owner
	// (no machine retry exists — the reconciliation is a human decision).
	t.Run("row-23 evidence delivery_unknown: unknown stays with the owner", func(t *testing.T) {
		unknownReply := replyOf(2)
		if _, err := evidence(unknownReply, casesDomain.DeliveryEvidenceDeliveryUnknown, "matrix-degradation-evidence-2"); err != nil {
			t.Fatal(err)
		}
		delivery, known := deliveryOf(unknownReply)
		if !known || delivery.State != casesDomain.ReplyDeliveryUnknown || delivery.LastKind != casesDomain.DeliveryEvidenceDeliveryUnknown {
			t.Fatalf("the ambiguous observation keeps the honest unknown, got %+v", delivery)
		}
		// No machine retry: a re-send is a NEW reply row (the human
		// decision) — nothing dispatches by itself here.
		if _, err := evidence(unknownReply, casesDomain.DeliveryEvidenceDeliveredConfirmed, "matrix-degradation-evidence-2"); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
			t.Fatalf("the same key with another material refuses with the conflict, got %v", err)
		}
	})

	// Row 24 — the terminal provider refusal: the evidence
	// provider_failed derives the failed state (the honest visible
	// refusal of the channel).
	t.Run("row-24 evidence provider_failed: the honest failed state", func(t *testing.T) {
		failedReply := replyOf(1)
		if _, err := evidence(failedReply, casesDomain.DeliveryEvidenceProviderFailed, "matrix-degradation-evidence-3"); err != nil {
			t.Fatal(err)
		}
		delivery, known := deliveryOf(failedReply)
		if !known || delivery.State != casesDomain.ReplyDeliveryFailed || delivery.LastKind != casesDomain.DeliveryEvidenceProviderFailed {
			t.Fatalf("the provider refusal derives the failed state, got %+v", delivery)
		}
	})

	// Row 25 — the monotonic history: several evidence events of ONE
	// reply — the event numbers grow monotonically; the observation
	// history stays readable; the LAST event decides the state (the
	// confirmed receipt followed by the ambiguous observation returns the
	// honest unknown — the last event wins even when it downgrades).
	t.Run("row-25 monotonic events: the last event decides, even a downgrade", func(t *testing.T) {
		multiReply := replyOf(0)
		if _, err := evidence(multiReply, casesDomain.DeliveryEvidenceDeliveredConfirmed, "matrix-degradation-evidence-4"); err != nil {
			t.Fatal(err)
		}
		if _, err := evidence(multiReply, casesDomain.DeliveryEvidenceDeliveryUnknown, "matrix-degradation-evidence-5"); err != nil {
			t.Fatal(err)
		}
		delivery, known := deliveryOf(multiReply)
		if !known || delivery.State != casesDomain.ReplyDeliveryUnknown || delivery.LastEventNo != 2 {
			t.Fatalf("the last event decides the state with the monotonic number, got %+v", delivery)
		}
		events, err := store.ReadDeliveryEvents(ctx, scope, caseID)
		if err != nil {
			t.Fatal(err)
		}
		own := eventsOfReply(events, multiReply)
		if len(own) != 2 || own[0].EventNo != 1 || own[1].EventNo != 2 {
			t.Fatalf("the observation history stays readable and monotonic, got %+v", own)
		}
	})

	// Row 26 — the outbox perimeter: the degradation envelope never
	// carries the note and the evidence envelope never carries the reply
	// content (the operational rows stay in the tables — only the
	// identifiers and the dictionary words travel).
	t.Run("row-26 outbox perimeter: note and content never leave the rows", func(t *testing.T) {
		var recordID id.UUID
		if err := adminPool.QueryRow(ctx, `
                        select record_id from cases.route_degradations
                        where tenant_id=$1 and case_id=$2 and revision=5`, w.tenant, caseID).Scan(&recordID); err != nil {
			t.Fatal(err)
		}
		var payload string
		if err := adminPool.QueryRow(ctx, `
                        select payload from cases.outbox_events
                        where tenant_id=$1 and dedup_key=$2`,
			w.tenant, casesDomain.DedupKeyRouteDegradationRecorded(recordID)).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, "канал снова молчит") || strings.Contains(payload, "note") {
			t.Fatalf("the degradation envelope never carries the note, got %s", payload)
		}
		confirmedReply := replyOf(3)
		var eventID id.UUID
		if err := adminPool.QueryRow(ctx, `
                        select event_id from cases.client_reply_delivery_events
                        where tenant_id=$1 and reply_id=$2 and event_no=1`, w.tenant, confirmedReply).Scan(&eventID); err != nil {
			t.Fatal(err)
		}
		var evidencePayload string
		if err := adminPool.QueryRow(ctx, `
                        select payload from cases.outbox_events
                        where tenant_id=$1 and dedup_key=$2`,
			w.tenant, casesDomain.DedupKeyClientReplyDeliveryEvidence(eventID)).Scan(&evidencePayload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(evidencePayload, "продолжим") || strings.Contains(evidencePayload, "content") {
			t.Fatalf("the evidence envelope never carries the reply content, got %s", evidencePayload)
		}
		var envelope casesDomain.ClientReplyDeliveryEvidencePayload
		if err := json.Unmarshal([]byte(evidencePayload), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.ReplyID != confirmedReply || envelope.EventNo != 1 || envelope.Kind != casesDomain.DeliveryEvidenceDeliveredConfirmed {
			t.Fatalf("the evidence envelope carries the reply/event/kind metadata, got %+v", envelope)
		}
	})

	// Row 27 — the transfer card: the pending request carries the current
	// degraded route words to the receiving operator BEFORE the ack (the
	// 2-4 card material of the SAME view — the preferences precedent of
	// 2-11); the second operator acknowledges — the epoch grows.
	t.Run("row-27 transfer card: the degradations are visible to the receiver before ack", func(t *testing.T) {
		request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
			ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
			WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "матричная строка 27: смена владельца на деградированном маршруте",
			IdempotencyKey: "matrix-degradation-transfer-1", Now: now,
		})
		if err != nil {
			t.Fatal(err)
		}
		view := viewOf()
		if view.Ownership.State.Pending == nil || view.Ownership.State.Pending.RequestID != request.TransferRequestID {
			t.Fatalf("the pending request is visible before the ack, got %+v", view.Ownership.State.Pending)
		}
		if len(view.RouteDegradations.Current) != 1 || view.RouteDegradations.Current[0].Revision != 5 {
			t.Fatalf("the SAME view carries the current degraded route of the card (the receiver material), got %+v", view.RouteDegradations.Current)
		}
		if _, err := w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: request.TransferRequestID,
			Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю владение",
			IdempotencyKey: "matrix-degradation-transfer-ack", Now: now,
		}); err != nil {
			t.Fatal(err)
		}
		after := viewOf()
		if after.Ownership.State.Current == nil || after.Ownership.State.Current.Epoch != 2 {
			t.Fatalf("the acknowledged transfer grows the epoch, got %+v", after.Ownership.State.Current)
		}
	})

	// Row 28 — the journeys chain: the row IS the spec.ts chain of the
	// real transport (the acceptance of step-04 — the chain runs in the
	// playwright contour, not here).
	t.Run("row-28 journeys chain (the spec.ts run)", func(t *testing.T) {
		t.Skip("the journeys chain zzz(22)-cases-degradation.spec.ts walks this row on the real transport")
	})
}
