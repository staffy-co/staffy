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

	casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/jackc/pgx/v5/pgconn"
)

// commitmentSeedSeq keeps every seeded commitment-surface row globally
// unique across the whole suite (the objection precedent: the ids of the
// direct admin inserts never collide with the command-path ids).
var commitmentSeedSeq int64

// nextCommitmentProbeID mints one globally unique probe id (the
// 019a2f4c-3000-8000 family — far from the command-path id space).
func nextCommitmentProbeID() id.UUID {
	commitmentSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-3000-7000-8000-%012d", 98000+commitmentSeedSeq))
}

// The Story 2.9 step-02 probes: the migration 00009 invariants are pinned
// by the live probes immediately (the review R1 lesson of 2-7 — the
// invariant is never only a domain guard). Three probe families: the
// pg_class/RLS/grants discipline of the four append-only tables, the
// INSERT backstops (the dictionary CHECK words, the shape CHECKs, the
// pair-CHECK of the client replies, the state-CHECK of the ladder, the
// subject UNIQUE, the outbox/command dictionary ALTERs) — the honest rows
// land first, so every refusal below isolates exactly its dishonest part.

// TestCommitmentTablesGrantsAndRLS asserts the append-only discipline of
// the commitment surface at the database level: RLS + FORCE on the four
// tables of 00009, INSERT/SELECT for staffy_app and no UPDATE, DELETE or
// TRUNCATE (the observation/objection precedent of 2.7/2.8).
func TestCommitmentTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('commitments','confirmations','commitment_declines','commitment_hmac_keys')`)
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
	for _, table := range []string{"commitments", "confirmations", "commitment_declines", "commitment_hmac_keys"} {
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

// commitmentProbeRow carries the honest template of one commitment
// version row with the per-probe dishonest knobs.
type commitmentProbeRow struct {
	commitmentID id.UUID
	versionNo    int64
	status       string
	conditions   string
	actionSet    string
	unknowns     string
	limit        int
	token        string
	ownerKind    string
	ownerEpoch   int64
	pinnedObs    int64
}

// honestCommitmentRow is the honest template (the domain-canonical shape
// of the material conditions, the dictionary words, the 64-char token).
func honestCommitmentRow(commitmentID id.UUID) commitmentProbeRow {
	return commitmentProbeRow{
		commitmentID: commitmentID,
		versionNo:    1,
		status:       casesDomain.CommitmentStatusOffered,
		conditions:   `{"consequences":"без изменений","next_step":"запись на сервис","payee":"operator","price":{"cents":500000,"kind":"amount"},"scope":"замена масла","service_items":["масло 5 л"],"warranty_role":"none"}`,
		actionSet:    `["prepare_order"]`,
		unknowns:     `[]`,
		limit:        1,
		token:        strings.Repeat("a", 64),
		ownerKind:    "human",
		ownerEpoch:   1,
		pinnedObs:    0,
	}
}

// insertCommitmentRow runs the direct INSERT of one commitment version
// row over the live case + work item anchors (the admin role behind the
// RLS curtain — the DB-probe precedent of 2.7/2.8).
func (w *casesWorld) insertCommitmentRow(t *testing.T, caseID, workItemID id.UUID, row commitmentProbeRow) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.commitments (tenant_id, case_id, work_item_id, commitment_id, version_no, status,
                        conditions, action_set, valid_until, consumption_limit, material_unknowns,
                        owner_kind, owner_ref, owner_epoch, lineage_word, policy_outcome_no, pinned_price_observation_no,
                        content_token, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,$9,$10,$11::jsonb,$12,$13,$14,'employee/1/1',0,$15,$16,$17,$18)`,
		w.tenant, caseID, workItemID, row.commitmentID, row.versionNo, row.status,
		row.conditions, row.actionSet, ownershipNow.Add(24*time.Hour), row.limit, row.unknowns,
		row.ownerKind, w.ownershipActor().ID, row.ownerEpoch, row.pinnedObs,
		row.token, w.ownershipActor().ID, ownershipNow)
	return err
}

// confirmationProbeRow carries the honest template of one confirmation
// row over the honest commitment anchor.
type confirmationProbeRow struct {
	confirmationID id.UUID
	subjectKind    string
	subjectRef     string
	method         string
	channel        string
	consumed       int
	limit          int
	spans          string
	snapshot       string
	actionSnapshot string
	token          string
}

// honestConfirmationRow is the honest template of the confirmation.
func honestConfirmationRow(confirmationID id.UUID) confirmationProbeRow {
	return confirmationProbeRow{
		confirmationID: confirmationID,
		subjectKind:    "client",
		subjectRef:     "client+79000000137",
		method:         casesDomain.CommitmentMethodExplicitText,
		channel:        "web_chat",
		consumed:       0,
		limit:          1,
		spans:          `["подтверждаю замену масла"]`,
		snapshot:       `{"scope":"замена масла"}`,
		actionSnapshot: `["prepare_order"]`,
		token:          strings.Repeat("b", 64),
	}
}

// insertConfirmationRow runs the direct INSERT of one confirmation row
// over the commitment anchor of the probe.
func (w *casesWorld) insertConfirmationRow(t *testing.T, caseID, workItemID, commitmentID id.UUID, row confirmationProbeRow) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.confirmations (tenant_id, case_id, work_item_id, commitment_id, version_no, confirmation_id,
                        subject_kind, subject_ref, method, channel, expires_at, consumed_count, consumption_limit, turn_ref,
                        evidence_spans, material_snapshot, action_set_snapshot, content_token, actor_id, confirmed_at)
                values ($1,$2,$3,$4,1,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14::jsonb,$15::jsonb,$16::jsonb,$17,$18,$19)`,
		w.tenant, caseID, workItemID, commitmentID, row.confirmationID,
		row.subjectKind, row.subjectRef, row.method, row.channel,
		ownershipNow.Add(24*time.Hour), row.consumed, row.limit, nextCommitmentProbeID(),
		row.spans, row.snapshot, row.actionSnapshot, row.token, w.ownershipActor().ID, ownershipNow)
	return err
}

// TestCommitmentDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00009 append-only surface: the dictionary CHECK words
// of the row (the status, the actions, the unknown terms, the method, the
// subject kind, the decline outcome), the shape CHECKs (the object
// conditions ≤4096, the bounded spans and action sets, the 64-char
// tokens, the zero consumption), the pair-CHECK of the client replies,
// the state-CHECK of the confirmed ladder word, the subject UNIQUE and
// the outbox/command dictionary ALTERs — the honest shape is a database
// guarantee, not only a domain guard (the review R1 mutation lesson of
// 2-7: the invariants are pinned by the live probes immediately). The
// direct INSERTs run as the admin role behind the RLS curtain.
func TestCommitmentDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 970)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-commitment-2-9-1", "sess-commitment-2-9-1", `{"call":"inbound-call","from":"+79000000137"}`)
	w.modeTurnEvent(t, caseID, "sess-commitment-2-9-1", "evt-commitment-2-9-1-turn-1", `{"say":"да, подтверждаю"}`)

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
	// checkDuplicate pins the unique_violation refusal of the second row
	// of the same subject of the same version.
	checkDuplicate := func(probe string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the duplicate subject row must be refused, got accepted", probe)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23505" {
			t.Fatalf("%s: expected the unique_violation 23505, got %v", probe, err)
		}
	}
	// commitmentProbe runs one dishonest commitment insert over the fresh
	// honest template (a fresh commitment id keeps the probes
	// independent; only the dishonest knob differs).
	commitmentProbe := func(mutate func(*commitmentProbeRow)) error {
		row := honestCommitmentRow(nextCommitmentProbeID())
		mutate(&row)
		return w.insertCommitmentRow(t, caseID, workItemID, row)
	}
	// confirmationProbe runs one dishonest confirmation insert over the
	// honest commitment anchor.
	anchor := honestCommitmentRow(nextCommitmentProbeID())
	if err := w.insertCommitmentRow(t, caseID, workItemID, anchor); err != nil {
		t.Fatalf("the honest commitment row must land, got %v", err)
	}
	confirmationProbe := func(mutate func(*confirmationProbeRow)) error {
		row := honestConfirmationRow(nextCommitmentProbeID())
		mutate(&row)
		return w.insertConfirmationRow(t, caseID, workItemID, anchor.commitmentID, row)
	}

	// The commitments: the honest row lands (the template is valid, so
	// every refusal below isolates exactly its dishonest part).
	if err := commitmentProbe(func(*commitmentProbeRow) {}); err != nil {
		t.Fatalf("the honest commitment row must land, got %v", err)
	}
	// The dictionary words of the row are the CHECK backstops.
	checkRefused("status outside the dictionary", commitmentProbe(func(r *commitmentProbeRow) { r.status = "magic_status" }))
	checkRefused("version_no below one", commitmentProbe(func(r *commitmentProbeRow) { r.versionNo = 0 }))
	checkRefused("owner kind outside the dictionary", commitmentProbe(func(r *commitmentProbeRow) { r.ownerKind = "operator" }))
	checkRefused("owner epoch below one", commitmentProbe(func(r *commitmentProbeRow) { r.ownerEpoch = 0 }))
	checkRefused("pinned observation below zero", commitmentProbe(func(r *commitmentProbeRow) { r.pinnedObs = -1 }))
	// The material conditions stay a bounded object.
	checkRefused("conditions not an object", commitmentProbe(func(r *commitmentProbeRow) { r.conditions = `["scope"]` }))
	checkRefused("conditions over 4096 bytes", commitmentProbe(func(r *commitmentProbeRow) {
		r.conditions = `{"pad":"` + strings.Repeat("A", 4100) + `"}`
	}))
	// The action set stays a non-empty bounded subset of the dictionary.
	checkRefused("empty action set", commitmentProbe(func(r *commitmentProbeRow) { r.actionSet = `[]` }))
	checkRefused("action outside the dictionary", commitmentProbe(func(r *commitmentProbeRow) { r.actionSet = `["magic_action"]` }))
	checkRefused("action set over four words", commitmentProbe(func(r *commitmentProbeRow) {
		r.actionSet = `["prepare_order","schedule_appointment","issue_document","prepare_order","schedule_appointment"]`
	}))
	// The unknown words stay the mandatory term words only.
	checkRefused("unknown term outside the mandatory dictionary", commitmentProbe(func(r *commitmentProbeRow) { r.unknowns = `["magic_term"]` }))
	// The consumption limit stays 1..10; the token stays 64 chars.
	checkRefused("consumption limit below one", commitmentProbe(func(r *commitmentProbeRow) { r.limit = 0 }))
	checkRefused("consumption limit over ten", commitmentProbe(func(r *commitmentProbeRow) { r.limit = 11 }))
	checkRefused("content token of the wrong length", commitmentProbe(func(r *commitmentProbeRow) { r.token = strings.Repeat("a", 63) }))

	// The confirmations: the honest row lands over the anchor.
	if err := confirmationProbe(func(*confirmationProbeRow) {}); err != nil {
		t.Fatalf("the honest confirmation row must land, got %v", err)
	}
	// The dictionary words and the consumption state are the CHECK
	// backstops; the snapshots keep the bounded shapes.
	checkRefused("method outside the dictionary", confirmationProbe(func(r *confirmationProbeRow) { r.method = "voice_readback" }))
	checkRefused("subject kind outside the dictionary", confirmationProbe(func(r *confirmationProbeRow) { r.subjectKind = "operator" }))
	checkRefused("consumed count above zero", confirmationProbe(func(r *confirmationProbeRow) { r.consumed = 1 }))
	checkRefused("consumption limit below one", confirmationProbe(func(r *confirmationProbeRow) { r.limit = 0 }))
	checkRefused("empty spans array", confirmationProbe(func(r *confirmationProbeRow) { r.spans = `[]` }))
	checkRefused("spans not an array", confirmationProbe(func(r *confirmationProbeRow) { r.spans = `"подтверждаю"` }))
	many := make([]string, 0, 9)
	for i := 0; i < 9; i++ {
		many = append(many, fmt.Sprintf("выдержка %d", i))
	}
	manyJSON, marshalErr := json.Marshal(many)
	if marshalErr != nil {
		t.Fatalf("the nine-span probe must marshal: %v", marshalErr)
	}
	checkRefused("more than eight spans", confirmationProbe(func(r *confirmationProbeRow) { r.spans = string(manyJSON) }))
	checkRefused("material snapshot not an object", confirmationProbe(func(r *confirmationProbeRow) { r.snapshot = `["scope"]` }))
	checkRefused("content token of the wrong length", confirmationProbe(func(r *confirmationProbeRow) { r.token = strings.Repeat("b", 63) }))
	// One confirmation per subject per version: the second row of the
	// SAME subject refuses (the fail-closed duplicate backstop).
	checkDuplicate("the duplicate subject of the same version", confirmationProbe(func(r *confirmationProbeRow) {
		r.subjectRef = "client+79000000137"
	}))

	// The declines: the honest row lands; the outcome word and the spans
	// shape are the CHECK backstops.
	insertDecline := func(outcome, spans string) error {
		_, err := adminPool.Exec(ctx, `
                        insert into cases.commitment_declines (tenant_id, case_id, work_item_id, commitment_id, version_no, decline_id,
                                outcome, turn_ref, evidence_spans, actor_id, created_at)
                        values ($1,$2,$3,$4,1,$5,$6,$7,$8::jsonb,$9,$10)`,
			w.tenant, caseID, workItemID, anchor.commitmentID, nextCommitmentProbeID(),
			outcome, nextCommitmentProbeID(), spans, w.ownershipActor().ID, ownershipNow)
		return err
	}
	if err := insertDecline(casesDomain.CommitmentDeclineOutcomeDeclined, `["нет, спасибо"]`); err != nil {
		t.Fatalf("the honest decline row must land, got %v", err)
	}
	checkRefused("decline outcome outside the dictionary", insertDecline("magic_outcome", `["нет"]`))
	checkRefused("decline spans not an array", insertDecline(casesDomain.CommitmentDeclineOutcomeChangeRequested, `"нет"`))
	checkRefused("empty decline spans", insertDecline(casesDomain.CommitmentDeclineOutcomeDeclined, `[]`))

	// The per-tenant commitment key: 32 bytes exactly (the ensure
	// pattern anchor of the content binding; the probe tenants never
	// collide with a world tenant — the short-key probe runs on its own
	// tenant so the length CHECK is the only backstop that fires).
	probeTenantA := id.MustParse("019a2f4c-4000-7000-8000-000000000099")
	probeTenantB := id.MustParse("019a2f4c-4000-7000-8000-000000000098")
	if _, err := adminPool.Exec(ctx, `
                insert into cases.commitment_hmac_keys (tenant_id, key, created_at) values ($1,$2,$3)`,
		probeTenantA, make([]byte, 32), ownershipNow); err != nil {
		t.Fatalf("the honest 32-byte key row must land, got %v", err)
	}
	_, keyErr := adminPool.Exec(ctx, `
                insert into cases.commitment_hmac_keys (tenant_id, key, created_at) values ($1,$2,$3)`,
		probeTenantB, make([]byte, 31), ownershipNow)
	checkRefused("the commitment key under 32 bytes", keyErr)

	// The presentation pair of the client replies: the nullable pair is
	// both or neither — the half pair refuses, the honest pair lands, the
	// legacy NULLs stay legal (the policy_outcome_id precedent of 2-6).
	conversationID := func() id.UUID {
		var conversation id.UUID
		if err := adminPool.QueryRow(ctx, `
                        select conversation_id from cases.conversations
                        where tenant_id=$1 and case_id=$2 order by conversation_no limit 1`,
			w.tenant, caseID).Scan(&conversation); err != nil {
			t.Fatal(err)
		}
		return conversation
	}()
	insertReply := func(commitmentID any, version any) error {
		_, err := adminPool.Exec(ctx, `
                        insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id,
                                owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction,
                                employee_id, decision_no, profile_version, content, actor_id, created_at,
                                commitment_id, commitment_version)
                        values ($1,$2,$3,$4,'human',$5,1,'inbox',$6,'synthetic','web_chat','outbound',$7,1,1,$8,$9,$10,$11,$12)`,
			w.tenant, caseID, conversationID, nextCommitmentProbeID(),
			w.ownershipActor().ID, w.connection, w.employeeID,
			"материал предложения версии", w.ownershipActor().ID, ownershipNow, commitmentID, version)
		return err
	}
	if err := insertReply(anchor.commitmentID, int64(1)); err != nil {
		t.Fatalf("the honest presentation pair must land, got %v", err)
	}
	if err := insertReply(nil, nil); err != nil {
		t.Fatalf("the honest legacy NULL pair must land, got %v", err)
	}
	checkRefused("the half pair (id without version)", insertReply(anchor.commitmentID, nil))
	checkRefused("the half pair (version without id)", insertReply(nil, int64(1)))

	// The confirmed word of the work item ladder: the additive 2-2
	// extension is a database word — the confirmed version row lands over
	// the live anchor, the magic state refuses (the pre-existing words
	// stay untouched — the control branch of the domain tests).
	insertState := func(versionNo int64, state string) error {
		_, err := adminPool.Exec(ctx, `
                        insert into cases.work_item_versions (tenant_id, case_id, conversation_id, work_item_id, version_no,
                                kind, type, title, state, reason, actor_id, ref, created_at)
                        values ($1,$2,$3,$4,$5,'transition',null,null,$6,'commitment_confirmed',$7,null,$8)`,
			w.tenant, caseID, conversationID, workItemID, versionNo, state, w.ownershipActor().ID, ownershipNow)
		return err
	}
	currentVersion := int64(1)
	if err := adminPool.QueryRow(ctx, `
                select max(version_no) from cases.work_item_versions
                where tenant_id=$1 and case_id=$2 and work_item_id=$3`,
		w.tenant, caseID, workItemID).Scan(&currentVersion); err != nil {
		t.Fatal(err)
	}
	if err := insertState(currentVersion+1, casesDomain.WorkItemStateConfirmed); err != nil {
		t.Fatalf("the confirmed state word must land (the ladder extension), got %v", err)
	}
	checkRefused("the state outside the ladder dictionary", insertState(currentVersion+2, "magic_state"))

	// The three outbox event types of 00009 are the dictionary words: the
	// commitment event lands, the magic type refuses (the envelope
	// carries the metadata only — the material never enters).
	insertOutbox := func(eventType string) error {
		_, err := adminPool.Exec(ctx, `
                        insert into cases.outbox_events (tenant_id, event_id, event_type, schema_version, aggregate_id,
                                source_connection_id, correlation_id, occurred_at, payload, dedup_key, created_at)
                        values ($1,$2,$3,1,$4,$5,$6,$7,'{"kind":"probe"}'::jsonb,$8,$9)`,
			w.tenant, nextCommitmentProbeID(), eventType, caseID, w.connection, nextCommitmentProbeID(),
			ownershipNow, fmt.Sprintf("commitment-probe-%d", commitmentSeedSeq), ownershipNow)
		return err
	}
	if err := insertOutbox("case.commitment_version_created"); err != nil {
		t.Fatalf("the commitment_version_created event type must land, got %v", err)
	}
	if err := insertOutbox("case.commitment_confirmed"); err != nil {
		t.Fatalf("the commitment_confirmed event type must land, got %v", err)
	}
	if err := insertOutbox("case.commitment_declined"); err != nil {
		t.Fatalf("the commitment_declined event type must land, got %v", err)
	}
	checkRefused("the event type outside the dictionary", insertOutbox("case.magic_event"))

	// The three command words of 00009 join the idempotency surface
	// dictionary: the create_commitment record lands, the magic command
	// refuses.
	insertCommand := func(command string) error {
		_, err := adminPool.Exec(ctx, `
                        insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                        values ($1,$2,$3,$4,$5,'{"ok":true}'::jsonb,$6)`,
			w.tenant, w.ownershipActor().ID, command,
			fmt.Sprintf("commitment-probe-%d", commitmentSeedSeq), strings.Repeat("c", 64), ownershipNow)
		return err
	}
	if err := insertCommand("create_commitment"); err != nil {
		t.Fatalf("the create_commitment command word must land, got %v", err)
	}
	checkRefused("the command outside the dictionary", insertCommand("magic_command"))
}
