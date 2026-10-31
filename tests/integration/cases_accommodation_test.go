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
	// The 2.12 mechanical extension: the v3 word is the honest rule of the
	// CURRENT slice (the constant) — the probe pins the FUTURE word refusal
	// instead (the historical v1/v2/v3 rows coexist, a next version word
	// never lands before its story).
	checkRefused("the future rule version word", w.insertPolicyOutcomeRow(t, caseID, workItemID, result.EventID, 46, "intake-policy-v4", "no_alternative_path", `[]`))
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

// TestAccommodationServiceCycleCoversDemoPath covers the Story 2.11 demo
// path through the REAL service: the record command (the scope gate
// BEFORE the guards — the audit-only member and the foreign tenant
// probes; the honest record cycle with the revision derivation; the
// replay idempotency and the command key conflict), the policy wiring
// (the recorded no_photo + the missing mandatory media-usual fact → the
// outcome carries the active alternatives of the pack and the preferences
// map of the input snapshot; the same anchors + the same preferences →
// the same digest; the clearing revision → the honest supersede without
// the alternatives), the accuracy gate of the sales reply (the missing
// qualifier against the alternatives — the honest required; the foreign
// word — the subset of the OUTCOME; the honest pass with the reply row
// carrying the word; the control branch — the empty word of the outcome
// without alternatives passes, the non-empty refuses), and the case view
// section (the current set, the honest history, the transfer card
// material of the receiving operator).
func TestAccommodationServiceCycleCoversDemoPath(t *testing.T) {
	w := newCasesWorld(t, 988)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-accommodation-svc-1", "sess-accommodation-svc-1", `{"call":"inbound-call","from":"+79000000141"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-accommodation-svc-1", "evt-accommodation-svc-1-turn-1", `{"say":"не могу отправлять фото, сколько стоит деталь"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "accommodation-sales-1")

	record := func(key, value, note string, cleared bool, keyNo string, actor identityDomain.Principal, tenant id.UUID) (casesAPI.WorkOutcome, error) {
		return w.cases.RecordAccommodation(ctx, casesAPI.RecordAccommodationCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID,
			AccommodationKey: key, AccommodationValue: value, Cleared: cleared, Note: note,
			IdempotencyKey: keyNo, Now: ownershipNow,
		})
	}

	// The scope gate BEFORE the material guards: the audit-only member and
	// the foreign tenant never write (the honest 403/404 words; the
	// dictionary violation never masks the scope refusal — the M1 gate
	// order).
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "accommodation-auditor-988@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000009886", "000000009889")
	auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
	if _, err := record(casesDomain.AccommodationNoPhoto, "", "", false, "accommodation-audit-1", auditOnly, w.tenant); !errors.Is(err, identityDomain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
		t.Fatalf("expected the honest 403 scope refusal, got %v", err)
	}
	if _, err := record("magic_key", "", "", false, "accommodation-audit-2", auditOnly, w.tenant); !errors.Is(err, identityDomain.ErrForbidden) {
		t.Fatalf("the dictionary violation never masks the scope refusal, got %v", err)
	}
	if _, err := record(casesDomain.AccommodationNoPhoto, "", "", false, "accommodation-foreign-1", w.ownershipActor(), id.MustParse("01994f64-6c00-7000-8000-0000000000f1")); !errors.Is(err, casesDomain.ErrNotFound) && !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("expected the honest 404 of the foreign tenant, got %v", err)
	}

	// The honest record cycle: the client says «не могу отправлять фото» —
	// the operator records no_photo (the revision 1).
	first, err := record(casesDomain.AccommodationNoPhoto, "", "клиент сказал: не могу фото", false, "accommodation-record-1", w.ownershipActor(), w.tenant)
	if err != nil || first.Command != "record_accommodation" || !first.AccommodationRecordID.Valid() || first.AccommodationKey != casesDomain.AccommodationNoPhoto {
		t.Fatalf("the honest record must land, got %+v %v", first, err)
	}
	// The exact repeat: the replay of the stored outcome (the idempotency).
	replayed, err := record(casesDomain.AccommodationNoPhoto, "", "клиент сказал: не могу фото", false, "accommodation-record-1", w.ownershipActor(), w.tenant)
	if err != nil || !replayed.Replay || replayed.AccommodationRecordID != first.AccommodationRecordID {
		t.Fatalf("the exact repeat replays the stored outcome, got %+v %v", replayed, err)
	}
	// The same key with ANOTHER material: the honest command key conflict.
	if _, err := record(casesDomain.AccommodationNoVoice, "", "", false, "accommodation-record-1", w.ownershipActor(), w.tenant); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material must refuse with the command key conflict, got %v", err)
	}
	// The dictionary guard of the service: the unknown key and the flag with
	// a value refuse honestly (the domain truth before SQL).
	if _, err := record("magic_key", "", "", false, "accommodation-guard-1", w.ownershipActor(), w.tenant); err == nil || !strings.Contains(err.Error(), "не входит в словарь") {
		t.Fatalf("the unknown key refuses with the dictionary words, got %v", err)
	}
	if _, err := record(casesDomain.AccommodationNoWeb, "yes", "", false, "accommodation-guard-2", w.ownershipActor(), w.tenant); err == nil || !strings.Contains(err.Error(), "не несёт значение") {
		t.Fatalf("the flag with a value refuses honestly, got %v", err)
	}

	// The view: the current set carries no_photo; the history carries the
	// honest revision 1 (the transfer card material of the receiving
	// operator — the same view scope).
	view, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Accommodations.Current) != 1 || view.Accommodations.Current[0].Key != casesDomain.AccommodationNoPhoto || view.Accommodations.Current[0].Cleared {
		t.Fatalf("the current set carries the active no_photo, got %+v", view.Accommodations.Current)
	}
	if len(view.Accommodations.History) != 1 || view.Accommodations.RecordsCount != 1 {
		t.Fatalf("the history carries the one honest revision, got %+v", view.Accommodations.History)
	}

	// The policy wiring: the port submits the proposal (the mandatory
	// media-usual service_kind fact is NOT confirmed) and the evaluation
	// runs with the recorded no_photo — the outcome carries the active
	// alternatives of the pack and the preferences map of the snapshot.
	firstOutcome := w.policyPrelude(t, caseID, conversationID, workItemID, "accommodation-policy-1")
	if firstOutcome.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
		t.Fatalf("expected the commercial outcome of the prelude, got %+v", firstOutcome)
	}
	currentOf := func() *casesDomain.PolicyOutcome {
		view, viewErr := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
		if viewErr != nil {
			t.Fatal(viewErr)
		}
		for _, section := range view.Policy.Items {
			if section.WorkItemID == workItemID {
				return section.Current
			}
		}
		return nil
	}
	withPreferences := currentOf()
	if withPreferences == nil || len(withPreferences.ActiveAlternatives) != len(casesDomain.AlternativePathWords) {
		t.Fatalf("the outcome carries the full pack alternatives, got %+v", withPreferences)
	}
	if _, known := withPreferences.InputSnapshot.Preferences[casesDomain.AccommodationNoPhoto]; !known {
		t.Fatalf("the input snapshot carries the preferences map, got %+v", withPreferences.InputSnapshot.Preferences)
	}

	// The reproducibility: the SAME proposal + the same anchors + the same
	// preferences state → the same decision digest (the honest second row
	// supersedes by the number, never the digest).
	digestFirst := withPreferences.DecisionDigest
	repeatedEvaluate := func() {
		t.Helper()
		fresh, viewErr := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
		if viewErr != nil {
			t.Fatal(viewErr)
		}
		proposalID := id.UUID("")
		version := int64(0)
		for _, section := range fresh.Policy.Items {
			if section.WorkItemID != workItemID {
				continue
			}
			for _, proposal := range section.Proposals {
				proposalID = proposal.ProposalID
			}
		}
		for _, item := range fresh.WorkItems {
			if item.WorkItemID == workItemID {
				version = item.Version
			}
		}
		if _, err := w.cases.EvaluatePolicy(ctx, casesAPI.EvaluatePolicyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposalID,
			ExpectedModeVersion: fresh.Policy.ModeVersion, ExpectedOwnerEpoch: fresh.Policy.OwnerEpoch,
			ExpectedFactsDigest: fresh.Policy.FactsDigest, ExpectedWorkItemVersion: version,
			IdempotencyKey: fmt.Sprintf("accommodation-reevaluate-%d", fresh.Policy.ModeVersion), Now: ownershipNow,
		}); err != nil {
			t.Fatal(err)
		}
	}
	repeatedEvaluate()
	if repeated := currentOf(); repeated.DecisionDigest != digestFirst || repeated.OutcomeNo <= withPreferences.OutcomeNo {
		t.Fatalf("the same inputs produce the same digest with the grown number, got %+v", repeated)
	}

	// The accuracy gate of the sales reply: the CURRENT outcome carries the
	// active alternatives — the reply without the qualifier refuses with
	// the honest required words.
	reply := func(qualifier, keyNo string) (casesAPI.WorkOutcome, error) {
		return w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
			AccuracyQualifier: qualifier, Content: "Сориентирую по описанию", IdempotencyKey: keyNo, Now: ownershipNow,
		})
	}
	if _, err := reply("", "accommodation-reply-required-1"); err == nil || !strings.Contains(err.Error(), casesDomain.AccuracyQualifierRequiredText[:40]) {
		t.Fatalf("the reply without the qualifier refuses with the required words, got %v", err)
	}
	// The word outside the active set of the outcome: the honest foreign
	// (the subset of the OUTCOME, never the dictionary in general — the
	// word IS a dictionary word but the outcome's set decides).
	if _, err := reply("magic_word", "accommodation-reply-foreign-1"); err == nil || !strings.Contains(err.Error(), "не входит в активные альтернативы") {
		t.Fatalf("the word outside the active set refuses honestly, got %v", err)
	}
	// The honest pass: the reply carries the qualifier word; the row keeps
	// it (the panel truth).
	qualified, err := reply(casesDomain.AlternativeTextDescription, "accommodation-reply-pass-1")
	if err != nil || qualified.ReplyState != "prepared" || qualified.AccuracyQualifier != casesDomain.AlternativeTextDescription {
		t.Fatalf("the qualified sales reply passes carrying the word, got %+v %v", qualified, err)
	}
	// The non-sales class with the qualifier word: the honest 400 of the
	// dictionary (the field has exactly one meaning).
	if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		AccuracyQualifier: casesDomain.AlternativeTextDescription, Content: "Нейтральное сообщение", IdempotencyKey: "accommodation-reply-neutral-1", Now: ownershipNow,
	}); err == nil || !strings.Contains(err.Error(), "несёт только коммерческий ответ") {
		t.Fatalf("the neutral class with the qualifier refuses with the dictionary words, got %v", err)
	}

	// The clearing revision: the client learned to send photos — the no_photo
	// leaves the current set (the honest history keeps both rows).
	cleared, err := record(casesDomain.AccommodationNoPhoto, "", "клиент научился присылать фото", true, "accommodation-record-2", w.ownershipActor(), w.tenant)
	if err != nil || cleared.AccommodationKey != casesDomain.AccommodationNoPhoto || !cleared.AccommodationCleared {
		t.Fatalf("the clearing revision must land, got %+v %v", cleared, err)
	}
	view, err = w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Accommodations.Current) != 0 || len(view.Accommodations.History) != 2 {
		t.Fatalf("the cleared key leaves the current set, the history keeps both rows, got %+v", view.Accommodations)
	}

	// The honest supersede: the re-evaluation over the cleared preferences
	// produces the outcome WITHOUT the alternatives and a NEW digest (the
	// changed preferences are the honest input change); the control branch
	// returns — the sales reply of the alternatives-free outcome passes
	// with the EMPTY word and refuses with any word.
	w.policyPrelude(t, caseID, conversationID, workItemID, "accommodation-policy-3")
	without := currentOf()
	if without == nil || len(without.ActiveAlternatives) != 0 {
		t.Fatalf("the cleared preferences produce the outcome without the alternatives, got %+v", without)
	}
	if without.DecisionDigest == digestFirst {
		t.Fatalf("the changed preferences change the digest (the honest supersede), got %s", without.DecisionDigest)
	}
	if _, err := reply(casesDomain.AlternativeTextDescription, "accommodation-reply-control-1"); err == nil || !strings.Contains(err.Error(), casesDomain.AccuracyQualifierForeignText[:40]) {
		t.Fatalf("the word against the alternatives-free outcome refuses with the foreign words, got %v", err)
	}
	control, err := reply("", "accommodation-reply-control-2")
	if err != nil || control.ReplyState != "prepared" || control.AccuracyQualifier != "" {
		t.Fatalf("the empty word of the alternatives-free outcome passes the old path, got %+v %v", control, err)
	}
}

// TestAccommodationMatrixRowsBySpec walks the I/O & Edge-Case Matrix of
// the spec row by row (26 named subtests over one wide matrix world —
// the continuation precedent of 2.10): the record rows (the append-only
// revisions, the valued keys, the guard dictionary, the replay/conflict
// pair, the rights gates, the terminal boundary), the policy rows (the
// alternatives wiring, the byte-identical control branch, the confirmed
// fact, the ask plan, the unit-level fail-closed pointer), the reply
// gate rows (the required/foreign/pass trio, the no-alternatives
// control, the non-sales dictionary), the digest rows (the
// reproducibility and the honest supersede), the transfer card row, the
// perimeter row, the RLS row, the continuation hint row and the journeys
// pointer.
func TestAccommodationMatrixRowsBySpec(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	w := newCasesWorld(t, 989)
	// The matrix world carries its own wide id sequence (the 2.10
	// precedent: the many-commanded rows drink the generator dry — the
	// 64-value worlds of the single-row families run out).
	matrixIDs := &sequenceIDs{values: make([]id.UUID, 512)}
	for i := range matrixIDs.values {
		matrixIDs.values[i] = id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 993000+i))
	}
	w.cases = w.buildCasesServiceClock(matrixIDs, fixedClock{ownershipNow})
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-accommodation-matrix-1", "sess-accommodation-matrix-1", `{"call":"inbound-call","from":"+79000000161"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-accommodation-matrix-1", "evt-accommodation-matrix-turn-1", `{"say":"не могу отправлять фото, сколько стоит деталь"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "matrix-accommodation-sales")
	now := ownershipNow

	// The record shorthand of the rows (the honest template; the knobs
	// travel through the key word, the value, the note, the clearing mark
	// and the command key).
	record := func(key, value, note string, cleared bool, keyNo string, actor identityDomain.Principal, tenant id.UUID) (casesAPI.WorkOutcome, error) {
		return w.cases.RecordAccommodation(ctx, casesAPI.RecordAccommodationCommand{
			Actor: actor, TenantID: tenant, CaseID: caseID,
			AccommodationKey: key, AccommodationValue: value, Cleared: cleared, Note: note,
			IdempotencyKey: keyNo, Now: now,
		})
	}
	// The view shorthand: the case view of the matrix case.
	viewOf := func() casesAPI.CaseView {
		detail, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		return detail
	}
	// The current outcome of the Work Item of one case (the maximal
	// number — the derivation of the reply gate).
	currentOf := func(caseNo, item id.UUID) *casesDomain.PolicyOutcome {
		detail, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseNo)
		if err != nil {
			t.Fatal(err)
		}
		for _, section := range detail.Policy.Items {
			if section.WorkItemID == item {
				return section.Current
			}
		}
		return nil
	}
	// The re-evaluation of the SAME proposal with a fresh command key
	// (the digest rows: the anchors never move, only the preferences or
	// the key change — the honest material of the row).
	reevaluate := func(caseNo, item id.UUID, keyNo string) {
		t.Helper()
		detail, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseNo)
		if err != nil {
			t.Fatal(err)
		}
		proposalID := id.UUID("")
		for _, section := range detail.Policy.Items {
			if section.WorkItemID != item {
				continue
			}
			for _, proposal := range section.Proposals {
				proposalID = proposal.ProposalID
			}
		}
		version := int64(0)
		for _, wi := range detail.WorkItems {
			if wi.WorkItemID == item {
				version = wi.Version
			}
		}
		if _, err := w.cases.EvaluatePolicy(ctx, casesAPI.EvaluatePolicyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseNo, ProposalID: proposalID,
			ExpectedModeVersion: detail.Policy.ModeVersion, ExpectedOwnerEpoch: detail.Policy.OwnerEpoch,
			ExpectedFactsDigest: detail.Policy.FactsDigest, ExpectedWorkItemVersion: version,
			IdempotencyKey: keyNo, Now: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// The facts digest of the empty fact world — the perimeter row
	// compares it after every record (the preferences never enter the
	// digest).
	digestBefore := viewOf().Policy.FactsDigest

	// The control worlds of the policy rows (row-11/13 read the world C
	// WITHOUT the preferences; row-12/18 read the world D with the
	// confirmed service_kind): created up front, the preludes run inside
	// their rows.
	caseC, workItemC := w.acceptedCase(t, "evt-accommodation-matrix-3", "sess-accommodation-matrix-3", `{"call":"inbound-call","from":"+79000000163"}`)
	conversationC := w.conversationFirst(t, caseC)
	turnC := w.modeTurnEvent(t, caseC, "sess-accommodation-matrix-3", "evt-accommodation-matrix-turn-3", `{"say":"сколько стоит замена детали"}`)
	caseD, workItemD := w.acceptedCase(t, "evt-accommodation-matrix-4", "sess-accommodation-matrix-4", `{"call":"inbound-call","from":"+79000000164"}`)
	conversationD := w.conversationFirst(t, caseD)
	turnD := w.modeTurnEvent(t, caseD, "sess-accommodation-matrix-4", "evt-accommodation-matrix-turn-4", `{"say":"сколько стоит замена детали"}`)

	// Row 1 — the honest record of no_photo (the revision 1): the
	// append-only row with the pins, the outbox envelope with the
	// metadata only (the value and the note never leave the row) and the
	// panel derivation of the current set (AC-1).
	t.Run("row-01 record: append-only row + outbox without value/note + panel current set", func(t *testing.T) {
		first, err := record(casesDomain.AccommodationNoPhoto, "", "клиент сказал: не могу фото", false, "matrix-accommodation-record-1", w.ownershipActor(), w.tenant)
		if err != nil || first.Command != "record_accommodation" || first.AccommodationKey != casesDomain.AccommodationNoPhoto || first.AccommodationCleared {
			t.Fatalf("the honest record must land with the command material, got %+v %v", first, err)
		}
		var revision int64
		var note, rowKey string
		var cleared bool
		if err := adminPool.QueryRow(ctx, `
                        select accommodation_key, revision, cleared, note from cases.case_accommodations
                        where tenant_id=$1 and case_id=$2 and record_id=$3`,
			w.tenant, caseID, first.AccommodationRecordID).Scan(&rowKey, &revision, &cleared, &note); err != nil {
			t.Fatal(err)
		}
		if rowKey != casesDomain.AccommodationNoPhoto || revision != 1 || cleared || note != "клиент сказал: не могу фото" {
			t.Fatalf("the append-only row pins the key/revision/cleared/note, got %s rev=%d cleared=%v note=%q", rowKey, revision, cleared, note)
		}
		var payload string
		if err := adminPool.QueryRow(ctx, `
                        select payload from cases.outbox_events
                        where tenant_id=$1 and dedup_key=$2`, w.tenant, casesDomain.DedupKeyAccommodationRecorded(first.AccommodationRecordID)).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, "клиент сказал") {
			t.Fatalf("the outbox envelope never carries the note, got %s", payload)
		}
		var envelope casesDomain.AccommodationRecordedPayload
		if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Key != casesDomain.AccommodationNoPhoto || envelope.Revision != 1 || envelope.Cleared {
			t.Fatalf("the envelope carries the key/revision/cleared metadata, got %+v", envelope)
		}
		view := viewOf()
		if len(view.Accommodations.Current) != 1 || view.Accommodations.Current[0].Key != casesDomain.AccommodationNoPhoto || view.Accommodations.Current[0].Revision != 1 {
			t.Fatalf("the panel current set carries the active no_photo, got %+v", view.Accommodations.Current)
		}
		if len(view.Accommodations.History) != 1 || view.Accommodations.RecordsCount != 1 {
			t.Fatalf("the history carries the one honest revision, got %+v", view.Accommodations.History)
		}
	})

	// Row 2 — the valued key preferred_channel: the channel word lands in
	// the row and the panel shows the word with the value; the outbox
	// envelope carries the metadata only (the value never travels).
	t.Run("row-02 preferred_channel: value saved + panel word=value + outbox without value", func(t *testing.T) {
		outcome, err := record(casesDomain.AccommodationPreferredChannel, "phone", "клиент просит звонить", false, "matrix-accommodation-record-2", w.ownershipActor(), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		var value *string
		if err := adminPool.QueryRow(ctx, `
                        select value from cases.case_accommodations
                        where tenant_id=$1 and case_id=$2 and record_id=$3`,
			w.tenant, caseID, outcome.AccommodationRecordID).Scan(&value); err != nil {
			t.Fatal(err)
		}
		if value == nil || *value != "phone" {
			t.Fatalf("the valued row stores the channel word, got %v", value)
		}
		var payload string
		if err := adminPool.QueryRow(ctx, `
                        select payload from cases.outbox_events
                        where tenant_id=$1 and dedup_key=$2`, w.tenant, casesDomain.DedupKeyAccommodationRecorded(outcome.AccommodationRecordID)).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, "phone") || strings.Contains(payload, "клиент просит") {
			t.Fatalf("the outbox envelope never carries the value or the note, got %s", payload)
		}
		view := viewOf()
		found := false
		for _, state := range view.Accommodations.Current {
			if state.Key == casesDomain.AccommodationPreferredChannel && state.Value == "phone" && state.Revision == 1 {
				found = true
			}
		}
		if !found {
			t.Fatalf("the panel current set carries the word with the value, got %+v", view.Accommodations.Current)
		}
	})

	// Row 3 — the valued key available_time: the free operational line
	// (1..100 runes, never a schedule) lands in the row; the value never
	// enters the outbox.
	t.Run("row-03 available_time: operational line stored + never in outbox", func(t *testing.T) {
		outcome, err := record(casesDomain.AccommodationAvailableTime, "после 18:00", "", false, "matrix-accommodation-record-3", w.ownershipActor(), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		var value *string
		if err := adminPool.QueryRow(ctx, `
                        select value from cases.case_accommodations
                        where tenant_id=$1 and case_id=$2 and record_id=$3`,
			w.tenant, caseID, outcome.AccommodationRecordID).Scan(&value); err != nil {
			t.Fatal(err)
		}
		if value == nil || *value != "после 18:00" {
			t.Fatalf("the availability line stays in the row, got %v", value)
		}
		var payload string
		if err := adminPool.QueryRow(ctx, `
                        select payload from cases.outbox_events
                        where tenant_id=$1 and dedup_key=$2`, w.tenant, casesDomain.DedupKeyAccommodationRecorded(outcome.AccommodationRecordID)).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(payload, "18:00") {
			t.Fatalf("the free line never enters the outbox, got %s", payload)
		}
		view := viewOf()
		found := false
		for _, state := range view.Accommodations.Current {
			if state.Key == casesDomain.AccommodationAvailableTime && state.Value == "после 18:00" {
				found = true
			}
		}
		if !found {
			t.Fatalf("the panel carries the availability line, got %+v", view.Accommodations.Current)
		}
	})

	// Row 4 — the guard dictionary of the record command: the key
	// dictionary, the flag/valued shape, the channel word, the bounds and
	// the clearing discipline refuse with the honest 400 rule words
	// BEFORE any SQL (the domain truth).
	t.Run("row-04 guards: key/flag/valued/channel/bounds/clearing 400 by domain", func(t *testing.T) {
		for _, probe := range []struct {
			name  string
			key   string
			value string
			clean bool
			note  string
			word  string
		}{
			{"unknown-key", "magic_key", "", false, "", "не входит в словарь"},
			{"flag-with-value", casesDomain.AccommodationNoWeb, "yes", false, "", "не несёт значение"},
			{"valued-without-value", casesDomain.AccommodationPreferredChannel, "", false, "", "требует значение"},
			{"foreign-channel-word", casesDomain.AccommodationPreferredChannel, "fax", false, "", "словарь каналов"},
			{"availability-too-long", casesDomain.AccommodationAvailableTime, strings.Repeat("с", 101), false, "", "1..100"},
			{"note-too-long", casesDomain.AccommodationNoVoice, "", false, strings.Repeat("н", 501), "длиннее 500"},
			{"cleared-with-value", casesDomain.AccommodationPreferredChannel, "phone", true, "", "нельзя одновременно"},
		} {
			_, err := record(probe.key, probe.value, probe.note, probe.clean, "matrix-accommodation-guard-"+probe.name, w.ownershipActor(), w.tenant)
			assertRuleError(t, err, probe.word)
		}
	})

	// Row 5 — the repeat record of the same key with ANOTHER material
	// (another command key): the revision 2 lands append-only; the
	// revision 1 stays readable; the current set derives the revision 2
	// («редактируемые» — AC-1).
	t.Run("row-05 repeat record: revision 2 append-only + history readable", func(t *testing.T) {
		if _, err := record(casesDomain.AccommodationNoVoice, "", "первое упоминание", false, "matrix-accommodation-record-4", w.ownershipActor(), w.tenant); err != nil {
			t.Fatal(err)
		}
		second, err := record(casesDomain.AccommodationNoVoice, "", "повторный акцент", false, "matrix-accommodation-record-5", w.ownershipActor(), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		var revisions int64
		if err := adminPool.QueryRow(ctx, `
                        select count(*) from cases.case_accommodations
                        where tenant_id=$1 and case_id=$2 and accommodation_key=$3`,
			w.tenant, caseID, casesDomain.AccommodationNoVoice).Scan(&revisions); err != nil {
			t.Fatal(err)
		}
		if revisions != 2 {
			t.Fatalf("the key carries two append-only revisions, got %d", revisions)
		}
		view := viewOf()
		current := false
		for _, state := range view.Accommodations.Current {
			if state.Key == casesDomain.AccommodationNoVoice && state.Revision == 2 {
				current = true
			}
		}
		if !current || second.AccommodationKey != casesDomain.AccommodationNoVoice {
			t.Fatalf("the current set derives the revision 2, got %+v", view.Accommodations.Current)
		}
	})

	// Row 6 — the clearing revision: the row lands honestly; the key
	// leaves the current set; the history shows the cleared row.
	t.Run("row-06 cleared revision: key leaves the current set + honest history", func(t *testing.T) {
		if _, err := record(casesDomain.AccommodationNoVoice, "", "клиент снова может говорить", true, "matrix-accommodation-record-6", w.ownershipActor(), w.tenant); err != nil {
			t.Fatal(err)
		}
		view := viewOf()
		for _, state := range view.Accommodations.Current {
			if state.Key == casesDomain.AccommodationNoVoice {
				t.Fatalf("the cleared key leaves the current set, got %+v", state)
			}
		}
		clearedRow := false
		for _, record := range view.Accommodations.History {
			if record.Key == casesDomain.AccommodationNoVoice && record.Revision == 3 && record.Cleared {
				clearedRow = true
			}
		}
		if !clearedRow || view.Accommodations.RecordsCount != 6 {
			t.Fatalf("the history keeps the cleared row honestly, got %+v (count %d)", view.Accommodations.History, view.Accommodations.RecordsCount)
		}
	})

	// Row 7 — the idempotent pair: the exact repeat of the row-1 command
	// replays the stored outcome; the same key with another material is
	// the honest command key conflict.
	t.Run("row-07 replay + key conflict 409", func(t *testing.T) {
		replayed, err := record(casesDomain.AccommodationNoPhoto, "", "клиент сказал: не могу фото", false, "matrix-accommodation-record-1", w.ownershipActor(), w.tenant)
		if err != nil || !replayed.Replay {
			t.Fatalf("the exact repeat replays the stored outcome, got %+v %v", replayed, err)
		}
		if _, err := record(casesDomain.AccommodationNoPhoto, "", "другая заметка той же команды", false, "matrix-accommodation-record-1", w.ownershipActor(), w.tenant); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
			t.Fatalf("the same key with another material refuses with the conflict, got %v", err)
		}
	})

	// Row 8 — the rights gates BEFORE the material guards (the M1
	// precedent): the audit-only member reads the scope words; the
	// dictionary violation never masks the scope refusal; the member of
	// another tenant reads the plain 404 (the RLS curtain).
	t.Run("row-08 gates: audit-only 403 BEFORE guards; foreign tenant 404", func(t *testing.T) {
		auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "matrix-accommodation-audit-989@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000009891", "000000009892")
		auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
		if _, err := record(casesDomain.AccommodationNoPhoto, "", "", false, "matrix-accommodation-audit-1", auditOnly, w.tenant); !errors.Is(err, identityDomain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("the audit-only member must be forbidden before the guards, got %v", err)
		}
		if _, err := record("magic_key", "", "", false, "matrix-accommodation-audit-2", auditOnly, w.tenant); !errors.Is(err, identityDomain.ErrForbidden) {
			t.Fatalf("the dictionary violation never masks the scope refusal, got %v", err)
		}
		foreignTenant := createForeignTenant(t, w)
		foreign := inviteMember(t, w.identity, w.owner, foreignTenant, "matrix-accommodation-foreign-989@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Review}, "000000009893", "000000009894")
		if _, err := record(casesDomain.AccommodationNoPhoto, "", "", false, "matrix-accommodation-foreign-1", foreign, w.tenant); !errors.Is(err, casesDomain.ErrNotFound) && !errors.Is(err, identityDomain.ErrNotFound) {
			t.Fatalf("the member of another tenant must read the plain 404, got %v", err)
		}
	})

	// Row 9 — the terminal case boundary: the record command refuses the
	// non-open case honestly (the app guard «обслуживать нечего»). The
	// v1 state dictionary of the slice carries the open word ONLY — the
	// terminal state word cannot exist in this slice (the DB CHECK is the
	// honest boundary; the app guard stays for the future dictionary
	// growth — the same honest shape as the pre-2.5 slices).
	t.Run("row-09 terminal case: the state dictionary keeps the boundary honest", func(t *testing.T) {
		terminalCase, _ := w.acceptedCase(t, "evt-accommodation-matrix-2", "sess-accommodation-matrix-2", `{"call":"inbound-call","from":"+79000000162"}`)
		_, err := adminPool.Exec(ctx, `
                        update cases.cases set state='resolved' where tenant_id=$1 and case_id=$2`,
			w.tenant, terminalCase)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("the terminal state word is impossible in this slice (the CHECK refusal), got %v", err)
		}
		// The open case still records (the boundary never blocks the
		// honest path).
		if _, err := w.cases.RecordAccommodation(ctx, casesAPI.RecordAccommodationCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: terminalCase,
			AccommodationKey: casesDomain.AccommodationTextFirst, AccommodationValue: "", Cleared: false, Note: "открытый кейс пишет",
			IdempotencyKey: "matrix-accommodation-record-open", Now: now,
		}); err != nil {
			t.Fatalf("the open case records honestly, got %v", err)
		}
	})

	// Row 10 — the wiring of the honest alternatives: the recorded
	// no_photo + the missing mandatory media-usual fact (service_kind)
	// under the sales mode → the outcome carries the full pack
	// alternatives; the question STAYS ask (an alternative never replaces
	// the question); the rule version v2; the input snapshot carries the
	// preferences map; the digest is deterministic (the row-20 proof).
	t.Run("row-10 wiring: alternatives + ask stays + v2 + snapshot preferences map", func(t *testing.T) {
		w.policyPrelude(t, caseID, conversationID, workItemID, "matrix-accommodation-policy-1")
		outcome := currentOf(caseID, workItemID)
		if outcome == nil || outcome.OutcomeClass != casesDomain.OutcomeCommercialAnswer {
			t.Fatalf("the commercial outcome of the prelude, got %+v", outcome)
		}
		if len(outcome.ActiveAlternatives) != len(casesDomain.AlternativePathWords) {
			t.Fatalf("the outcome carries the full pack alternatives, got %+v", outcome.ActiveAlternatives)
		}
		for _, word := range casesDomain.AlternativePathWords {
			if !inWords(outcome.ActiveAlternatives, word) {
				t.Fatalf("the alternative word %s must be active, got %+v", word, outcome.ActiveAlternatives)
			}
		}
		serviceKind := false
		for _, fact := range outcome.MandatoryFacts {
			if fact.MaterialKey == "service_kind" {
				if fact.State != casesDomain.MandatoryStateAsk {
					t.Fatalf("the media-usual question stays ask, got %+v", fact)
				}
				serviceKind = true
			}
		}
		if !serviceKind {
			t.Fatalf("the service_kind question never leaves the plan, got %+v", outcome.MandatoryFacts)
		}
		// The 2.12 mechanical extension: the rule word assert goes through the
		// DOMAIN CONSTANT (the honest current word — v3 after 2.12; the stored
		// outcome always carries the constant of its evaluation moment).
		if outcome.RuleVersion != casesDomain.PolicyRuleVersion {
			t.Fatalf("the rule version is the honest current word, got %s (constant %s)", outcome.RuleVersion, casesDomain.PolicyRuleVersion)
		}
		snapshot, known := outcome.InputSnapshot.Preferences[casesDomain.AccommodationNoPhoto]
		if !known || snapshot.Revision != 1 || snapshot.Cleared {
			t.Fatalf("the input snapshot carries the preferences map, got %+v", outcome.InputSnapshot.Preferences)
		}
	})

	// Row 11 — the control branch: the same evaluation WITHOUT the
	// recorded preferences → the decision fields (the class, the reason,
	// the mandatory plan, the next steps) are byte-identical and the
	// active alternatives stay EMPTY (the digest honestly differs — the
	// snapshot map is part of the v2 digest).
	t.Run("row-11 control branch: the decision is identical without preferences", func(t *testing.T) {
		w.policySalesPrelude(t, caseC, conversationC, turnC, "matrix-accommodation-control-sales")
		w.policyPrelude(t, caseC, conversationC, workItemC, "matrix-accommodation-control-1")
		with := currentOf(caseID, workItemID)
		without := currentOf(caseC, workItemC)
		if without == nil || len(without.ActiveAlternatives) != 0 {
			t.Fatalf("the control outcome carries no alternatives, got %+v", without)
		}
		if without.OutcomeClass != with.OutcomeClass || without.Reason != with.Reason {
			t.Fatalf("the class and the reason are identical, got %+v vs %+v", without, with)
		}
		statesOf := func(outcome *casesDomain.PolicyOutcome) map[string]string {
			states := map[string]string{}
			for _, fact := range outcome.MandatoryFacts {
				states[fact.MaterialKey] = fact.State
			}
			return states
		}
		if len(statesOf(without)) != len(statesOf(with)) {
			t.Fatalf("the mandatory plans are identical, got %v vs %v", statesOf(without), statesOf(with))
		}
		for key, state := range statesOf(with) {
			if statesOf(without)[key] != state {
				t.Fatalf("the mandatory plan of %s is identical, got %s vs %s", key, statesOf(without)[key], state)
			}
		}
		if len(without.NextSteps) != len(with.NextSteps) {
			t.Fatalf("the next steps are identical, got %+v vs %+v", without.NextSteps, with.NextSteps)
		}
		if without.DecisionDigest == with.DecisionDigest {
			t.Fatalf("the digest honestly differs (the preferences map is the v2 input), got the same %s", without.DecisionDigest)
		}
	})

	// Row 12 — the confirmed media-usual fact closes the trigger: the
	// no_photo stays recorded BUT service_kind is confirmed (the 2.6
	// dedup) → the fact is not missing → the outcome carries no
	// alternatives; the question is reused.
	t.Run("row-12 confirmed service_kind: no alternatives (the fact is not missing)", func(t *testing.T) {
		w.policySalesPrelude(t, caseD, conversationD, turnD, "matrix-accommodation-fact-sales")
		if _, err := w.cases.RecordAccommodation(ctx, casesAPI.RecordAccommodationCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseD,
			AccommodationKey: casesDomain.AccommodationNoPhoto, AccommodationValue: "", Cleared: false, Note: "клиент не может фото",
			IdempotencyKey: "matrix-accommodation-fact-1", Now: now,
		}); err != nil {
			t.Fatal(err)
		}
		w.recordConfirmedFact(t, caseD, workItemD, "service_kind", `"диагностика"`, "evt-accommodation-matrix-4", "matrix-accommodation-fact")
		w.policyPrelude(t, caseD, conversationD, workItemD, "matrix-accommodation-fact-2")
		outcome := currentOf(caseD, workItemD)
		if outcome == nil || len(outcome.ActiveAlternatives) != 0 {
			t.Fatalf("the confirmed fact closes the trigger, got %+v", outcome)
		}
		reused := false
		for _, fact := range outcome.MandatoryFacts {
			if fact.MaterialKey == "service_kind" && fact.State == casesDomain.MandatoryStateReused {
				reused = true
			}
		}
		if !reused {
			t.Fatalf("the confirmed fact is reused, got %+v", outcome.MandatoryFacts)
		}
	})

	// Row 13 — the ask control: the media-usual key missing with the
	// EMPTY preferences → the ask plan stays as before (the pre-2.11
	// path; the regression families of 2.6 stay byte-identical).
	t.Run("row-13 ask control: the question plan stays as before", func(t *testing.T) {
		outcome := currentOf(caseC, workItemC)
		ask := 0
		for _, fact := range outcome.MandatoryFacts {
			if fact.State == casesDomain.MandatoryStateAsk {
				ask++
			}
		}
		if ask == 0 || len(outcome.ActiveAlternatives) != 0 {
			t.Fatalf("the empty preferences keep the ask plan without the alternatives, got %+v", outcome)
		}
	})

	// Row 14 — the fail-closed stop of the EMPTY pack dictionary: the
	// unit-level probe (the pack dictionary is compile-time — the
	// integration contour cannot empty it; the domain family pins the
	// honest stop).
	t.Run("row-14 fail-closed empty pack (the unit probe)", func(t *testing.T) {
		t.Skip("the domain unit test TestApplyMediaAlternativesFailClosedOnEmptyPack walks this row (the pack dictionary is compile-time)")
	})

	// Row 15 — the accuracy gate: the reply without the qualifier against
	// the outcome WITH the active alternatives refuses with the honest
	// required words carrying the active set.
	t.Run("row-15 reply without qualifier: 400 required with the active words", func(t *testing.T) {
		_, err := sendMatrixReply(w, ctx, caseID, conversationID, workItemID, "", "matrix-accommodation-reply-required")
		assertRuleError(t, err, "обязан заявить ограничение точности")
		for _, word := range casesDomain.AlternativePathWords {
			if !strings.Contains(err.Error(), word) {
				t.Fatalf("the required refusal names the active words (%s), got %v", word, err)
			}
		}
	})

	// Row 16 — the qualifier outside the active set of the outcome: the
	// honest foreign (the subset of the OUTCOME, never the dictionary in
	// general).
	t.Run("row-16 foreign qualifier: 400 outside the active set", func(t *testing.T) {
		_, err := sendMatrixReply(w, ctx, caseID, conversationID, workItemID, "magic_word", "matrix-accommodation-reply-foreign")
		assertRuleError(t, err, "не входит в активные альтернативы")
	})

	// Row 17 — the honest pass: the reply carries the qualifier word of
	// the active set — the full ladder (ownership → mode → disclosure →
	// policy → objection → accuracy → CAS); the reply ROW carries the
	// word; the panel shows it.
	t.Run("row-17 qualified reply: passes + the row carries the word", func(t *testing.T) {
		outcome, err := sendMatrixReply(w, ctx, caseID, conversationID, workItemID, casesDomain.AlternativeTextDescription, "matrix-accommodation-reply-pass")
		if err != nil || outcome.ReplyState != "prepared" || outcome.AccuracyQualifier != casesDomain.AlternativeTextDescription {
			t.Fatalf("the qualified sales reply passes carrying the word, got %+v %v", outcome, err)
		}
		var qualifier *string
		if err := adminPool.QueryRow(ctx, `
                        select accuracy_qualifier from cases.client_replies
                        where tenant_id=$1 and reply_id=$2`, w.tenant, outcome.ReplyID).Scan(&qualifier); err != nil {
			t.Fatal(err)
		}
		if qualifier == nil || *qualifier != casesDomain.AlternativeTextDescription {
			t.Fatalf("the reply row keeps the qualifier word, got %v", qualifier)
		}
		view := viewOf()
		found := false
		for _, card := range view.Ownership.Replies {
			if card.Reply.ReplyID == outcome.ReplyID && card.Reply.AccuracyQualifier == casesDomain.AlternativeTextDescription {
				found = true
			}
		}
		if !found {
			t.Fatalf("the panel reply row carries the word, got %+v", view.Ownership.Replies)
		}
	})

	// Row 18 — the no-alternatives control: the reply with a non-empty
	// qualifier against the alternatives-free outcome refuses with the
	// foreign word; the empty word passes the old path.
	t.Run("row-18 no-alternatives control: foreign word 400; empty passes", func(t *testing.T) {
		_, err := sendMatrixReply(w, ctx, caseD, conversationD, workItemD, casesDomain.AlternativeTextDescription, "matrix-accommodation-reply-control-1")
		assertRuleError(t, err, "альтернатив в действующем исходе нет")
		control, err := sendMatrixReply(w, ctx, caseD, conversationD, workItemD, "", "matrix-accommodation-reply-control-2")
		if err != nil || control.ReplyState != "prepared" || control.AccuracyQualifier != "" {
			t.Fatalf("the empty word of the alternatives-free outcome passes the old path, got %+v %v", control, err)
		}
	})

	// Row 19 — the non-sales class with the qualifier word: the honest
	// 400 of the dictionary (the field has exactly one meaning).
	t.Run("row-19 non-sales with qualifier: 400 dictionary", func(t *testing.T) {
		_, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			AccuracyQualifier: casesDomain.AlternativeTextDescription, Content: "Нейтральное сообщение",
			IdempotencyKey: "matrix-accommodation-reply-neutral", Now: now,
		})
		assertRuleError(t, err, "несёт только коммерческий ответ")
	})

	// Row 20 — the reproducibility: the re-evaluation of the SAME
	// proposal with the same anchors and the UNCHANGED preferences → the
	// same decision digest (the number grows — the honest append-only).
	t.Run("row-20 reproducibility: same anchors + same preferences = same digest", func(t *testing.T) {
		before := currentOf(caseID, workItemID)
		reevaluate(caseID, workItemID, "matrix-accommodation-reevaluate-1")
		after := currentOf(caseID, workItemID)
		if after.DecisionDigest != before.DecisionDigest || after.OutcomeNo <= before.OutcomeNo {
			t.Fatalf("the same inputs produce the same digest with the grown number, got %s/%d after %s/%d",
				after.DecisionDigest, after.OutcomeNo, before.DecisionDigest, before.OutcomeNo)
		}
		if len(after.ActiveAlternatives) != len(casesDomain.AlternativePathWords) {
			t.Fatalf("the alternatives stay active, got %+v", after.ActiveAlternatives)
		}
	})

	// Row 21 — the honest supersede: the clearing revision of no_photo
	// between the evaluations → the new outcome carries ANOTHER digest
	// (the preferences map is the honest input change), no alternatives;
	// the number grows; the predecessor stays readable.
	t.Run("row-21 supersede: cleared preference changes the digest", func(t *testing.T) {
		if _, err := record(casesDomain.AccommodationNoPhoto, "", "клиент научился присылать фото", true, "matrix-accommodation-record-7", w.ownershipActor(), w.tenant); err != nil {
			t.Fatal(err)
		}
		before := currentOf(caseID, workItemID)
		reevaluate(caseID, workItemID, "matrix-accommodation-reevaluate-2")
		after := currentOf(caseID, workItemID)
		if after.DecisionDigest == before.DecisionDigest {
			t.Fatalf("the changed preferences change the digest (the honest supersede), got %s", after.DecisionDigest)
		}
		if len(after.ActiveAlternatives) != 0 {
			t.Fatalf("the cleared no_photo closes the trigger, got %+v", after.ActiveAlternatives)
		}
		detail, err := w.cases.CaseView(ctx, w.ownershipActor(), w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		outcomesOfItem := 0
		for _, section := range detail.Policy.Items {
			if section.WorkItemID == workItemID {
				outcomesOfItem = len(section.Outcomes)
			}
		}
		if outcomesOfItem < 3 {
			t.Fatalf("the predecessor outcomes stay readable (the append-only history), got %d", outcomesOfItem)
		}
	})

	// Row 22 — the transfer card: the pending request carries the current
	// preferences to the receiving operator BEFORE the ack (AC-1
	// «передаются каждому последующему Response Owner» — the transport
	// joins the ownership pending state with the accommodation current
	// set of the SAME view).
	t.Run("row-22 transfer card: the preferences are visible to the receiver before ack", func(t *testing.T) {
		request, err := w.cases.RequestTransfer(ctx, casesAPI.RequestTransferCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ExpectedEpoch: 1,
			ToOwnerKind: casesDomain.OwnerKindHuman, ToOwnerRef: w.backup,
			WritableSurface: casesDomain.WritableSurfaceInbox, Reason: "матричная строка 22: смена владельца",
			IdempotencyKey: "matrix-accommodation-transfer-1", Now: now,
		})
		if err != nil {
			t.Fatal(err)
		}
		view := viewOf()
		if view.Ownership.State.Pending == nil || view.Ownership.State.Pending.RequestID != request.TransferRequestID {
			t.Fatalf("the pending request is visible before the ack, got %+v", view.Ownership.State.Pending)
		}
		carriesChannel, carriesTime := false, false
		for _, state := range view.Accommodations.Current {
			if state.Key == casesDomain.AccommodationPreferredChannel && state.Value == "phone" {
				carriesChannel = true
			}
			if state.Key == casesDomain.AccommodationAvailableTime && state.Value == "после 18:00" {
				carriesTime = true
			}
		}
		if !carriesChannel || !carriesTime {
			t.Fatalf("the SAME view carries the current preferences of the card (the receiver material), got %+v", view.Accommodations.Current)
		}
		if _, err := w.cases.ResolveTransfer(ctx, casesAPI.ResolveTransferCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID, RequestID: request.TransferRequestID,
			Decision: casesDomain.TransferResolutionAcknowledged, Reason: "принимаю владение",
			IdempotencyKey: "matrix-accommodation-transfer-ack", Now: now,
		}); err != nil {
			t.Fatal(err)
		}
	})

	// Row 23 — the perimeter probe: the accommodation rows are NEVER
	// facts — no fact anchors, no fact versions, no verification records,
	// and the facts digest of the case never moved (the live probe of
	// AC-1 «не диагноз и не доказательство»).
	t.Run("row-23 perimeter: never facts, the digest never moves", func(t *testing.T) {
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
				t.Fatalf("%s: the accommodation rows never become the fact material, got %d", probe.word, rows)
			}
		}
		if digest := viewOf().Policy.FactsDigest; digest != digestBefore {
			t.Fatalf("the facts digest never moves (the preferences are not the fact material), got %s after %s", digest, digestBefore)
		}
	})

	// Row 24 — the RLS/grants discipline of the migration (the live
	// probe of the family above; the row keeps the traceable core).
	t.Run("row-24 RLS + INSERT/SELECT-only", func(t *testing.T) {
		var rls, force bool
		if err := runtimePool.QueryRow(ctx, `
                        select relrowsecurity, relforcerowsecurity from pg_class
                        where relnamespace='cases'::regnamespace and relname='case_accommodations'`).Scan(&rls, &force); err != nil {
			t.Fatal(err)
		}
		if !rls || !force {
			t.Fatalf("case_accommodations must carry RLS + FORCE, got %v/%v", rls, force)
		}
		for _, grant := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
			var allowed bool
			if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases.case_accommodations", grant).Scan(&allowed); err != nil {
				t.Fatal(err)
			}
			if allowed {
				t.Fatalf("%s on cases.case_accommodations must never be granted", grant)
			}
		}
	})

	// Row 25 — the continuation hint: the no_web/no_voice words of the
	// current set are the honest hint material of the continuation issue
	// form (the link IS a web page — the operator decides the surface);
	// the machine NEVER blocks the issuance without a capability
	// registry.
	t.Run("row-25 continuation hint: the words travel, the machine never blocks", func(t *testing.T) {
		if _, err := record(casesDomain.AccommodationNoWeb, "", "клиент не пользуется web", false, "matrix-accommodation-record-web", w.ownershipActor(), w.tenant); err != nil {
			t.Fatal(err)
		}
		if _, err := record(casesDomain.AccommodationNoVoice, "", "клиент снова не может говорить", false, "matrix-accommodation-record-voice", w.ownershipActor(), w.tenant); err != nil {
			t.Fatal(err)
		}
		view := viewOf()
		carriesWeb, carriesVoice := false, false
		for _, state := range view.Accommodations.Current {
			if state.Key == casesDomain.AccommodationNoWeb {
				carriesWeb = true
			}
			if state.Key == casesDomain.AccommodationNoVoice {
				carriesVoice = true
			}
		}
		if !carriesWeb || !carriesVoice {
			t.Fatalf("the current set carries the hint words, got %+v", view.Accommodations.Current)
		}
		issuance, err := w.cases.IssueContinuationLink(ctx, casesAPI.IssueContinuationLinkCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			Purposes: []string{casesDomain.ContinuationPurposeFactCorrection},
			Question: "Вопрос матричной строки 25", Explanation: "Матричная подсказка поверхности",
			ParticipantRef: "client+79000000161", TTLMinutes: 30,
			IdempotencyKey: "matrix-accommodation-issue-1", Now: now,
		})
		if err != nil || issuance.URL == "" || len(issuance.Token) != 64 {
			t.Fatalf("the issuance passes over the recorded no_web/no_voice (the machine never blocks), got %+v %v", issuance, err)
		}
	})

	// Row 26 — the journeys chain: the row IS the spec.ts chain of the
	// real transport (the acceptance of step-04 — the chain runs in the
	// playwright contour, not here).
	t.Run("row-26 journeys chain (the spec.ts run)", func(t *testing.T) {
		t.Skip("the journeys chain zzz(21)-cases-accommodation.spec.ts walks this row on the real transport")
	})
}

// sendMatrixReply is the sales-reply shorthand of the matrix rows (the
// qualifier knob of the accuracy gate).
func sendMatrixReply(w *casesWorld, ctx context.Context, caseID, conversationID, workItemID id.UUID, qualifier, keyNo string) (casesAPI.WorkOutcome, error) {
	return w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		AccuracyQualifier: qualifier, Content: "Сориентирую по описанию", IdempotencyKey: keyNo, Now: ownershipNow,
	})
}

// inWords reports whether the word belongs to the list.
func inWords(words []string, word string) bool {
	for _, candidate := range words {
		if candidate == word {
			return true
		}
	}
	return false
}
