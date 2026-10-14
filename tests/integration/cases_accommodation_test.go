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

// accommodationSeedSeq keeps every seeded accommodation-surface row
// globally unique across the whole suite (the continuation precedent).
var accommodationSeedSeq int64

// nextAccommodationProbeID mints one globally unique probe id.
func nextAccommodationProbeID() id.UUID {
	accommodationSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 99600+accommodationSeedSeq))
}

// The Story 2.11 step-02 probes: the migration 00011 invariants are
// pinned by the live probes immediately (the review R1 lesson of 2.7 —
// the invariant is never only a domain guard). Three probe families:
// the pg_class/RLS/grants discipline of the append-only table, the
// INSERT backstops (the key dictionary CHECK, the flag/valued shape
// CHECK, the channel word, the bounds, the clearing discipline, the
// revision UNIQUE, the v2 dictionaries of the policy outcomes, the
// accuracy qualifier word and the command/outbox ALTERs), and the
// store-level probes (the record command under the locks, the monotonic
// revision derivation, the append-only history, the read-time
// derivation, the honest outbox metadata and the perimeter absence in
// the fact channels).

// TestAccommodationTablesGrantsAndRLS asserts the append-only discipline
// of the accommodation surface at the database level: RLS + FORCE on
// case_accommodations, INSERT/SELECT for staffy_app and no UPDATE,
// DELETE or TRUNCATE (the continuation precedent of 2.10).
func TestAccommodationTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
		select relname, relrowsecurity, relforcerowsecurity from pg_class
		where relnamespace = 'cases'::regnamespace
		  and relname in ('case_accommodations')`)
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
	for _, table := range []string{"case_accommodations"} {
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

// accommodationProbeRow carries the honest template of one revision
// record with the per-probe dishonest knobs.
type accommodationProbeRow struct {
	recordID id.UUID
	key      string
	revision int64
	value    any
	cleared  bool
	note     string
}

// honestAccommodationRow is the honest template of the record (the
// dictionary key, the bounds, the flag shape).
func honestAccommodationRow(recordID id.UUID) accommodationProbeRow {
	return accommodationProbeRow{
		recordID: recordID,
		key:      casesDomain.AccommodationNoPhoto,
		revision: 1,
		value:    nil,
		cleared:  false,
		note:     "",
	}
}

// insertAccommodationRow runs the direct INSERT of one record row over
// the live case anchor (the admin role behind the RLS curtain — the
// DB-probe precedent of 2.7..2.10).
func (w *casesWorld) insertAccommodationRow(t *testing.T, caseID id.UUID, row accommodationProbeRow) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
		insert into cases.case_accommodations (tenant_id, case_id, record_id, accommodation_key, revision, value, cleared, note, actor_id, recorded_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		w.tenant, caseID, row.recordID, row.key, row.revision, row.value, row.cleared, row.note, w.ownershipActor().ID, ownershipNow)
	return err
}

// insertPolicyOutcomeRow runs the direct INSERT of one minimal policy
// outcome row over a directly-seeded proposal (the v2 dictionary
// probes — the FK chain: the case, the conversation, the work item of
// the world and the fresh proposal row).
func (w *casesWorld) insertPolicyOutcomeRow(t *testing.T, caseID, workItemID, turnRef id.UUID, outcomeNo int64, ruleVersion, reason, alternatives string) error {
	t.Helper()
	ctx := context.Background()
	conversationID := w.conversationFirst(t, caseID)
	proposalID := nextAccommodationProbeID()
	outcomeID := nextAccommodationProbeID()
	if _, err := adminPool.Exec(ctx, `
		insert into cases.ai_proposals (tenant_id, case_id, conversation_id, work_item_id, proposal_id, turn_ref,
			schema_version, intent, candidate_answer, missing_facts, extracted_facts, proposer, owner_epoch, mode_version, actor_id, created_at)
		values ($1,$2,$3,$4,$5,$6,'ai-proposal-v1','price_inquiry','','[]'::jsonb,'[]'::jsonb,'synthetic_port',1,1,$7,$8)`,
		w.tenant, caseID, conversationID, workItemID, proposalID, turnRef, w.ownershipActor().ID, ownershipNow); err != nil {
		return err
	}
	_, err := adminPool.Exec(ctx, `
		insert into cases.policy_outcomes (tenant_id, case_id, conversation_id, work_item_id, proposal_id, outcome_id,
			outcome_no, outcome_class, reason, rule_version, intent, active_alternatives, input_snapshot, decision_digest, actor_id, created_at)
		values ($1,$2,$3,$4,$5,$6,$12,'fail_closed',$7,$8,'price_inquiry',$9::jsonb,'{}'::jsonb,repeat('a',64),$10,$11)`,
		w.tenant, caseID, conversationID, workItemID, proposalID, outcomeID, reason, ruleVersion, alternatives, w.ownershipActor().ID, ownershipNow, outcomeNo)
	return err
}

// TestAccommodationDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00011 surface: the key dictionary CHECK, the
// flag/valued shape CHECK (the flag without a value, the channel word of
// preferred_channel, the 1..100 availability line), the clearing
// discipline (never together with a value), the note bound, the revision
// monotonicity UNIQUE, the v2 dictionaries of the policy outcomes (the
// rule_version v1/v2 words, the no_alternative_path reason, the
// alternative words subset), the accuracy qualifier word of the replies
// and the command/outbox ALTERs — the honest shape is a database
// guarantee, not only a domain guard. The direct INSERTs run as the
// admin role behind the RLS curtain.
func TestAccommodationDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 986)
	ctx := context.Background()
	outcome, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-accommodation-2-11-1", "sess-accommodation-2-11-1", `{"call":"inbound-call","from":"+79000000137"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := acceptedOutcome(t, outcome)
	caseID := result.CaseID
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.WorkItems) != 1 {
		t.Fatalf("expected the initial work item, got %d", len(detail.WorkItems))
	}
	workItemID := detail.WorkItems[0].WorkItemID

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
	// honest lands first: the honest flag record is a database-legal row.
	if err := w.insertAccommodationRow(t, caseID, honestAccommodationRow(nextAccommodationProbeID())); err != nil {
		t.Fatalf("the honest record must land, got %v", err)
	}
	// The honest valued records: the channel word and the availability line.
	if err := w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationPreferredChannel, revision: 1, value: "phone"}); err != nil {
		t.Fatalf("the preferred channel word must land, got %v", err)
	}
	if err := w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationAvailableTime, revision: 1, value: "после 18:00"}); err != nil {
		t.Fatalf("the availability line must land, got %v", err)
	}
	// The honest clearing revision: the flag removed (the value stays NULL).
	if err := w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationTextFirst, revision: 1}); err != nil {
		t.Fatalf("the flag record must land, got %v", err)
	}
	if err := w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationTextFirst, revision: 2, cleared: true}); err != nil {
		t.Fatalf("the clearing revision must land, got %v", err)
	}

	// The dictionary backstops of the record row.
	checkRefused("unknown key", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: "no_photos"}))
	checkRefused("flag with value", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationNoPhoto, value: "yes"}))
	checkRefused("valued without value", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationPreferredChannel}))
	checkRefused("foreign channel word", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationPreferredChannel, value: "fax"}))
	checkRefused("empty availability line", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationAvailableTime, value: ""}))
	checkRefused("101-rune availability line", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationAvailableTime, value: strings.Repeat("с", 101)}))
	checkRefused("clearing with value", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationPreferredChannel, value: "phone", cleared: true}))
	checkRefused("501-rune note", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationNoVoice, note: strings.Repeat("н", 501)}))
	// The revision monotonicity UNIQUE: the same (case, key, revision) pair
	// never repeats.
	checkDuplicate("duplicate revision", w.insertAccommodationRow(t, caseID, accommodationProbeRow{recordID: nextAccommodationProbeID(), key: casesDomain.AccommodationNoPhoto, revision: 1}))

	// The v2 dictionaries of the policy outcomes: the no_alternative_path
	// reason and the non-empty alternatives are the legal v2 form (the
	// control words stay legal — the v1 historical rows and the v2 rows
	// coexist).
	if err := w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 41, "intake-policy-v2", "no_alternative_path", `["text_description","range_estimate","callback","human_handoff"]`); err != nil {
		t.Fatalf("the v2 outcome with the alternatives must land, got %v", err)
	}
	if err := w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 42, "intake-policy-v1", "ok_commercial", `[]`); err != nil {
		t.Fatalf("the v1 historical words stay legal, got %v", err)
	}
	checkRefused("foreign alternative word", w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 43, "intake-policy-v2", "no_alternative_path", `["text_description","made_up_path"]`))
	checkRefused("five alternatives", w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 44, "intake-policy-v2", "no_alternative_path", `["text_description","range_estimate","callback","human_handoff","text_description"]`))
	checkRefused("null alternatives", w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 45, "intake-policy-v2", "no_alternative_path", `null`))
	checkRefused("v3 rule version", w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 46, "intake-policy-v3", "no_alternative_path", `[]`))
	checkRefused("foreign reason", w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 47, "intake-policy-v2", "made_up_reason", `[]`))

	// The accuracy qualifier word of the outgoing replies: the honest word
	// lands, the foreign word is refused (the playbook_step precedent).
	conversationID := w.conversationFirst(t, caseID)
	replyID := nextAccommodationProbeID()
	if _, err := adminPool.Exec(ctx, `
		insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch,
			writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version,
			content, accuracy_qualifier, actor_id, created_at)
		values ($1,$2,$3,$4,'human',$5,1,'inbox',$6,'web-chat','web_chat','outbound',$7,1,1,'Сориентирую по описанию',$8,$9,$10)`,
		w.tenant, caseID, conversationID, replyID, w.ownershipActor().ID, nextAccommodationProbeID(), w.ownershipActor().ID, casesDomain.AlternativeTextDescription, w.ownershipActor().ID, ownershipNow); err != nil {
		t.Fatalf("the qualified reply word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
		insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id, owner_kind, owner_ref, epoch,
			writable_surface, connection_id, provider, channel, direction, employee_id, decision_no, profile_version,
			content, accuracy_qualifier, actor_id, created_at)
		values ($1,$2,$3,$4,'human',$5,1,'inbox',$6,'web-chat','web_chat','outbound',$7,1,1,'Сориентирую по описанию',$8,$9,$10)`,
		w.tenant, caseID, conversationID, nextAccommodationProbeID(), w.ownershipActor().ID, nextAccommodationProbeID(), w.ownershipActor().ID, "made_up_qualifier", w.ownershipActor().ID, ownershipNow)
	checkRefused("foreign accuracy qualifier", err)

	// The command and outbox dictionary ALTERs: the record_accommodation
	// command word and the case.accommodation_recorded event type are the
	// legal dictionary words (the dishonest forms are refused).
	if _, err := adminPool.Exec(ctx, `
		insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
		values ($1,$2,'record_accommodation',$3,repeat('b',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, "accommodation-dict-probe-1", ownershipNow); err != nil {
		t.Fatalf("the record_accommodation command word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
		insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
		values ($1,$2,'record_preference',$3,repeat('b',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, "accommodation-dict-probe-2", ownershipNow)
	checkRefused("foreign command word", err)
	if _, err := adminPool.Exec(ctx, `
		insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
		values ($1,$2,'case.accommodation_recorded',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextAccommodationProbeID(), caseID, nextAccommodationProbeID(), nextAccommodationProbeID(), ownershipNow, "accommodation-outbox-probe-1", ownershipNow); err != nil {
		t.Fatalf("the case.accommodation_recorded event word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
		insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
		values ($1,$2,'case.preference_recorded',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextAccommodationProbeID(), caseID, nextAccommodationProbeID(), nextAccommodationProbeID(), ownershipNow, "accommodation-outbox-probe-2", ownershipNow)
	checkRefused("foreign outbox event word", err)
}

// TestAccommodationStoreRecordReads covers the store-level surface: the
// record command under the locks (the honest revision 1 with the outbox
// metadata), the monotonic revision derivation of the NEXT record, the
// append-only history, the read-time derivation of the CURRENT set, the
// domain guard refusals under the locks, the raced repeat (the stale
// signal), the key conflict of the same key with another material, the
// honest outbox envelope (the value and the note never enter) and the
// perimeter absence (no fact rows are ever written).
func TestAccommodationStoreRecordReads(t *testing.T) {
	w := newCasesWorld(t, 987)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-accommodation-store-1", "sess-accommodation-store-1", `{"call":"inbound-call","from":"+79000000137"}`)
	conversationID := w.conversationFirst(t, caseID)
	store := casesStore.NewPostgres(runtimePool)
	// The command-probe scope: the ACTOR is the principal (the
	// command_records RLS policy demands actor == principal).
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	recordParams := func(key string, recordID id.UUID, commandKey string, value string, cleared bool, note string) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: scope, Command: "record_accommodation", Key: commandKey,
			RequestHash: strings.Repeat("a", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, ConversationID: conversationID, WorkItemID: workItemID,
			AccommodationRecordID: recordID, AccommodationKey: key, AccommodationValue: value,
			AccommodationCleared: cleared, AccommodationNote: note,
			CorrelationID: nextAccommodationProbeID(), OutboxEventID: nextAccommodationProbeID(),
			Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}

	// The honest revision 1: the flag record lands with the outbox event.
	firstID := nextAccommodationProbeID()
	if err := store.ExecuteWorkCommand(ctx, recordParams(casesDomain.AccommodationNoPhoto, firstID, "accommodation-record-1", "", false, "клиент сказал: не могу фото")); err != nil {
		t.Fatalf("the honest record must land, got %v", err)
	}
	// The valued revision of another key: the channel word.
	secondID := nextAccommodationProbeID()
	if err := store.ExecuteWorkCommand(ctx, recordParams(casesDomain.AccommodationPreferredChannel, secondID, "accommodation-record-2", "phone", false, "")); err != nil {
		t.Fatalf("the valued record must land, got %v", err)
	}
	// The repeat of the same key: the revision 2 (the append-only history —
	// the store derives the next monotonic number under the locks).
	thirdID := nextAccommodationProbeID()
	if err := store.ExecuteWorkCommand(ctx, recordParams(casesDomain.AccommodationNoPhoto, thirdID, "accommodation-record-3", "", false, "повторный акцент")); err != nil {
		t.Fatalf("the revision 2 must land, got %v", err)
	}
	// The clearing revision: the key leaves the active set.
	fourthID := nextAccommodationProbeID()
	if err := store.ExecuteWorkCommand(ctx, recordParams(casesDomain.AccommodationNoPhoto, fourthID, "accommodation-record-4", "", true, "клиент научился присылать фото")); err != nil {
		t.Fatalf("the clearing revision must land, got %v", err)
	}

	// The read: every revision travels ordered by the key and the number;
	// the CURRENT set derives read-time (the cleared no_photo leaves the
	// active set, the preferred channel stays).
	records, err := store.ReadAccommodations(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 4 {
		t.Fatalf("expected the four revision records, got %d", len(records))
	}
	current := casesDomain.DeriveCurrentAccommodations(records)
	if current.HasActive(casesDomain.AccommodationNoPhoto) {
		t.Fatalf("the cleared no_photo is not active")
	}
	if state := current[casesDomain.AccommodationNoPhoto]; state.Revision != 3 || !state.Cleared {
		t.Fatalf("the cleared state carries the latest revision, got %+v", state)
	}
	if !current.HasActive(casesDomain.AccommodationPreferredChannel) {
		t.Fatalf("the preferred channel stays active")
	}

	// The guard probes under the locks (the honest 400 words of the domain
	// guard re-validated by the store).
	guardProbe := func(key, value string, cleared bool, note string, fragment string) {
		t.Helper()
		err := store.ExecuteWorkCommand(ctx, recordParams(key, nextAccommodationProbeID(), fmt.Sprintf("accommodation-guard-%d", accommodationSeedSeq), value, cleared, note))
		assertRuleError(t, err, fragment)
	}
	guardProbe("made_up_key", "", false, "", "не входит в словарь")
	guardProbe(casesDomain.AccommodationNoWeb, "yes", false, "", "не несёт значение")
	guardProbe(casesDomain.AccommodationPreferredChannel, "", false, "", "требует значение")
	guardProbe(casesDomain.AccommodationPreferredChannel, "fax", false, "", "словарь каналов")
	guardProbe(casesDomain.AccommodationAvailableTime, strings.Repeat("с", 101), false, "", "1..100")
	guardProbe(casesDomain.AccommodationNoVoice, "", false, strings.Repeat("н", 501), "длиннее 500")
	guardProbe(casesDomain.AccommodationPreferredChannel, "phone", true, "", "нельзя одновременно")

	// The raced repeat of the identical material (the same record id under
	// the same command key): the stale signal.
	if err := store.ExecuteWorkCommand(ctx, recordParams(casesDomain.AccommodationNoPhoto, firstID, "accommodation-record-1", "", false, "клиент сказал: не могу фото")); !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("the raced identical record must refuse with the stale signal, got %v", err)
	}
	// The same key with ANOTHER material: the honest command key conflict.
	conflictParams := recordParams(casesDomain.AccommodationNoVoice, nextAccommodationProbeID(), "accommodation-record-1", "", false, "")
	if err := store.ExecuteWorkCommand(ctx, conflictParams); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material must refuse with the command key conflict, got %v", err)
	}
	// The unknown case: the honest 404 (the non-participant path).
	unknownCase := recordParams(casesDomain.AccommodationNoPhoto, nextAccommodationProbeID(), "accommodation-record-unknown", "", false, "")
	unknownCase.CaseID = nextAccommodationProbeID()
	if err := store.ExecuteWorkCommand(ctx, unknownCase); !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the unknown case must refuse with the not-found word, got %v", err)
	}

	// The outbox honesty: the envelope of the honest record carries the
	// metadata only — the VALUE and the NOTE never enter (the sealed
	// material stays in the row).
	var payload string
	if err := adminPool.QueryRow(ctx, `
		select payload from cases.outbox_events
		where tenant_id=$1 and dedup_key=$2`, w.tenant, casesDomain.DedupKeyAccommodationRecorded(secondID)).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, "phone") || strings.Contains(payload, "клиент сказал") {
		t.Fatalf("the outbox envelope never carries the value or the note, got %s", payload)
	}
	var envelope casesDomain.AccommodationRecordedPayload
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Key != casesDomain.AccommodationPreferredChannel || envelope.Revision != 1 || envelope.Cleared {
		t.Fatalf("the outbox envelope carries the key/revision/cleared metadata, got %+v", envelope)
	}

	// The perimeter probe (the preferences are NEVER facts): the record
	// command wrote no fact rows — the fact channels stay absent for the
	// accommodation keys.
	var factRows int
	if err := adminPool.QueryRow(ctx, `
		select count(*) from cases.facts
		where tenant_id=$1 and case_id=$2 and material_key in ('no_photo','no_web','no_voice','text_first','slowed_pace','mandatory_readback','preferred_channel','available_time')`,
		w.tenant, caseID).Scan(&factRows); err != nil {
		t.Fatal(err)
	}
	if factRows != 0 {
		t.Fatalf("the accommodation keys never become fact anchors, got %d rows", factRows)
	}

	// The foreign-tenant scope: the RLS curtain — the read of another
	// tenant finds no rows (the honest empty answer, never a disclosure).
	foreignScope, err := tenancy.New(nextAccommodationProbeID(), id.MustParse("019a2f4c-6000-7000-8000-000000000098"))
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := store.ReadAccommodations(ctx, foreignScope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(foreign) != 0 {
		t.Fatalf("the foreign-tenant read finds no rows, got %d", len(foreign))
	}
}
