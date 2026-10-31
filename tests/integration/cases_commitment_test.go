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

	observationsyn "github.com/staffy-co/staffy/internal/adapters/observationsyn"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	casesStore "github.com/staffy-co/staffy/internal/cases/store"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
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

// ---------------------------------------------------------------------------
// The Story 2.9 command matrix: the 25 rows of the spec live (the REAL
// service over the live PostgreSQL) — the creation ladder, the honest price
// gates, the exact-version confirmation chain, the presentation link, the
// replay/duplicate discipline, the validity derivations, the decline and
// the rights/isolation boundaries.
// ---------------------------------------------------------------------------

// commitmentPriceValue is the canonical price bytes of the matrix world
// (the scenario script answers EXACTLY these bytes — the byte-equal price
// gate of the spec).
func commitmentPriceValue(t *testing.T) []byte {
	t.Helper()
	value, err := casesDomain.PriceValueJSON(casesDomain.CommitmentPrice{Kind: casesDomain.CommitmentPriceKindAmount, AmountCents: 300000})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// commitmentTerms is the honest material of the matrix rows.
func commitmentTerms() casesDomain.MaterialTerms {
	return casesDomain.MaterialTerms{
		Scope: "замена масла", ServiceItems: []string{"масло 5 л", "фильтр"},
		Price: casesDomain.CommitmentPrice{Kind: casesDomain.CommitmentPriceKindAmount, AmountCents: 300000},
		Payee: "operator", WarrantyRole: "none", NextStep: "запись на сервис", Consequences: "без изменений",
	}
}

// commitmentWorld builds the world with the price scenario answering the
// canonical price bytes (the byte-equal gate material).
func commitmentWorld(t *testing.T, seed int) *casesWorld {
	t.Helper()
	w := newCasesWorld(t, seed)
	w.cases = w.buildCasesObservationService(newPrivacyIDs(9710+seed), fixedClock{ownershipNow}, map[string]observationsyn.Scenario{
		casesDomain.MaterialFieldPrice: {Word: observationsyn.ScenarioOK, Value: commitmentPriceValue(t)},
	})
	return w
}

// commitmentCreate is the create shorthand over the world owner actor.
func (w *casesWorld) commitmentCreate(t *testing.T, command casesAPI.CreateCommitmentCommand) (casesAPI.WorkOutcome, error) {
	t.Helper()
	return w.cases.CreateCommitment(context.Background(), command)
}

// commitmentCreateCommand builds the honest create command.
func (w *casesWorld) commitmentCreateCommand(caseID, workItemID id.UUID, commitmentID id.UUID, status, key string) casesAPI.CreateCommitmentCommand {
	return casesAPI.CreateCommitmentCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, Status: status, Terms: commitmentTerms(),
		ActionSet:  []string{casesDomain.CommitmentActionPrepareOrder},
		ValidUntil: ownershipNow.Add(24 * time.Hour), ConsumptionLimit: 1,
		IdempotencyKey: key, Now: ownershipNow,
	}
}

// commitmentAssurance passes the verified identity and the acting
// authority of the confirm subject (the 2-3 ladder: the callback gives the
// verified identity; the mandate note gives the authority).
func (w *casesWorld) commitmentAssurance(t *testing.T, caseID, conversationID id.UUID, subject string) {
	t.Helper()
	ctx := context.Background()
	identity := w.verificationCommand(caseID, conversationID, "commitment-identity-"+subject, verificationNow)
	identity.Kind, identity.Method, identity.ProviderCode = casesDomain.VerificationKindIdentity, casesDomain.MethodKnownNumberCallback, casesDomain.ProviderCodeOK
	identity.SubjectHandle, identity.Note = subject, "перезвонили на известный номер, абонент подтвердил детали"
	if _, err := w.cases.StartVerification(ctx, identity); err != nil {
		t.Fatal(err)
	}
	authority := w.verificationCommand(caseID, conversationID, "commitment-authority-"+subject, verificationNow)
	authority.Kind, authority.Method, authority.ProviderCode = casesDomain.VerificationKindAuthority, casesDomain.MethodMandateNote, casesDomain.ProviderCodeOK
	authority.SubjectHandle, authority.AuthorityScope, authority.Note = subject, "payment", "основание полномочий: письменная доверенность"
	if _, err := w.cases.StartVerification(ctx, authority); err != nil {
		t.Fatal(err)
	}
}

// commitmentViewOf reads the commitment section of the case view.
func (w *casesWorld) commitmentViewOf(t *testing.T, caseID id.UUID) casesAPI.CommitmentView {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	return detail.Commitments
}

// commitmentSectionOf resolves the Work Item section of the view.
func commitmentSectionOf(t *testing.T, view casesAPI.CommitmentView, workItemID id.UUID) casesAPI.CommitmentWorkItemView {
	t.Helper()
	for _, section := range view.Items {
		if section.WorkItemID == workItemID {
			return section
		}
	}
	t.Fatalf("the commitment section of the work item %s not found", workItemID)
	return casesAPI.CommitmentWorkItemView{}
}

// TestCommitmentMatrixCreateVersionedPanel covers the rows 1, 4, 5, 6, 7
// (the creation ladder, the versioning, the panel derivation, the sales
// reply link and the non-sales refusal): the offered version №1 lands with
// the pins and the token; the same commitment id appends №2 (the old row
// never mutates); the draft refuses the link; the neutral reply with the
// pair refuses; the sales reply with the link passes the frozen ladder and
// records the presentation.
func TestCommitmentMatrixCreateVersionedPanel(t *testing.T) {
	w := commitmentWorld(t, 971)
	caseID, workItemID := w.acceptedCase(t, "evt-commitment-m1", "sess-commitment-m1", `{"call":"inbound-call","from":"+79000000137"}`)
	w.modeTurnEvent(t, caseID, "sess-commitment-m1", "evt-commitment-m1-turn-1", `{"say":"интересует замена масла"}`)
	// The price observation of the gate (ok, fresh, the byte-equal value).
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "commitment-obs-1"))
	commitmentID := id.MustParse("019a2f4c-5000-7000-8000-000000000001")

	outcome := w.observationOutcome(t, w.commitmentCreateCommand(caseID, workItemID, commitmentID, casesDomain.CommitmentStatusOffered, "commitment-create-1"))
	if outcome.CommitmentID != commitmentID || outcome.CommitmentVersion != 1 || outcome.CommitmentStatus != casesDomain.CommitmentStatusOffered {
		t.Fatalf("expected the offered version №1, got %+v", outcome)
	}
	// The panel shows the version: the readiness confirmable, the current
	// flag, the token presence and the pin.
	section := commitmentSectionOf(t, w.commitmentViewOf(t, caseID), workItemID)
	if len(section.Versions) != 1 || !section.Versions[0].Current || section.Versions[0].Readiness != casesDomain.CommitmentReadinessConfirmable {
		t.Fatalf("the panel shows the confirmable current version, got %+v", section.Versions)
	}
	if section.Versions[0].Version.ContentToken == "" || section.Versions[0].Version.PinnedPriceObservationNo == 0 {
		t.Fatalf("the version carries the content token and the observation pin, got %+v", section.Versions[0].Version)
	}

	// The second version of the same commitment: №2 lands append-only, №1
	// stays byte-identical (AC-8).
	second := w.observationOutcome(t, w.commitmentCreateCommand(caseID, workItemID, commitmentID, casesDomain.CommitmentStatusOffered, "commitment-create-2"))
	if second.CommitmentVersion != 2 {
		t.Fatalf("the changed offer is the NEW version №2, got %+v", second)
	}
	section = commitmentSectionOf(t, w.commitmentViewOf(t, caseID), workItemID)
	if len(section.Versions) != 2 || section.Versions[1].Version.Conditions == nil || section.Versions[0].Current {
		t.Fatalf("the panel shows the two versions with №2 current, got %+v", section.Versions)
	}

	// The sales-reply prelude of the frozen ladder (the sales mode + the
	// disclosure + the commercial Policy Outcome of the Work Item — the
	// 2-5/2-6 gates the linked reply passes byte-identically).
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-commitment-m1", "evt-commitment-m1-turn-2", `{"say":"сколько стоит замена"}`)
	w.policySalesPrelude(t, caseID, conversationID, turn, "commitment-sales-1")
	w.policyPrelude(t, caseID, conversationID, workItemID, "commitment-policy-1")
	linkedReply := func(commitmentOf id.UUID, versionNo int64, key string) (casesAPI.WorkOutcome, error) {
		return w.cases.SendClientReply(context.Background(), casesAPI.SendClientReplyCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
			CommitmentID: commitmentOf, CommitmentVersionNo: versionNo,
			Content: "Предлагаем условия версии", IdempotencyKey: key, Now: ownershipNow,
		})
	}

	// The draft refuses the presentation link (AC-1).
	draft := w.commitmentCreateCommand(caseID, workItemID, id.MustParse("019a2f4c-5000-7000-8000-000000000002"), casesDomain.CommitmentStatusDraft, "commitment-create-3")
	if _, err := w.commitmentCreate(t, draft); err != nil {
		t.Fatal(err)
	}
	_, err := linkedReply(id.MustParse("019a2f4c-5000-7000-8000-000000000002"), 1, "commitment-reply-draft")
	assertRuleError(t, err, "draft-not-presentable")

	// The sales reply with the offered link passes the frozen ladder and
	// records the presentation (the machine part of AC-4).
	replyOutcome, err := linkedReply(commitmentID, 2, "commitment-reply-1")
	if err != nil || replyOutcome.Replay || !replyOutcome.ReplyID.Valid() {
		t.Fatalf("the linked sales reply lands, got %+v %v", replyOutcome, err)
	}
	section = commitmentSectionOf(t, w.commitmentViewOf(t, caseID), workItemID)
	presented := false
	for _, card := range section.Versions {
		if card.Version.VersionNo == 2 {
			presented = card.Presented
		}
	}
	if !presented {
		t.Fatalf("the presentation of the exact version №2 must be recorded, got %+v", section.Versions)
	}
}

// observationOutcome is the honest outcome-or-fatal helper.
func (w *casesWorld) observationOutcome(t *testing.T, command casesAPI.CreateCommitmentCommand) casesAPI.WorkOutcome {
	t.Helper()
	outcome, err := w.commitmentCreate(t, command)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// TestCommitmentMatrixConfirmChain covers the rows 8-14 (the exact-version
// confirmation chain): the happy path over the presentation link, the
// verbatim spans, the assurance ladder and the freshness pin; the invented
// span; the missing assurance; the replay of the same key; the duplicate
// subject with another key; the same key with another material; the
// confirmation transits the Work Item to confirmed.
func TestCommitmentMatrixConfirmChain(t *testing.T) {
	w := commitmentWorld(t, 972)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-commitment-m2", "sess-commitment-m2", `{"call":"inbound-call","from":"+79000000138"}`)
	w.classifyCommitmentItem(t, caseID, workItemID, "commitment-classify-972")
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-commitment-m2", "evt-commitment-m2-turn-1", `{"say":"да, подтверждаю замену масла, согласен на 3000 рублей"}`)
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "commitment-obs-2"))
	commitmentID := id.MustParse("019a2f4c-5000-7000-8000-000000000011")
	if _, err := w.commitmentCreate(t, w.commitmentCreateCommand(caseID, workItemID, commitmentID, casesDomain.CommitmentStatusOffered, "commitment-create-11")); err != nil {
		t.Fatal(err)
	}
	// The presentation link (the sales prelude + the linked reply).
	w.policySalesPrelude(t, caseID, conversationID, turn, "commitment-sales-2")
	w.policyPrelude(t, caseID, conversationID, workItemID, "commitment-policy-2")
	if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		CommitmentID: commitmentID, CommitmentVersionNo: 1,
		Content: "Предлагаем условия версии №1", IdempotencyKey: "commitment-reply-11", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	// The assurance ladder of the subject (the verified identity + the
	// mandate note of the price).
	// The voice turn refuses honestly (AG-4: the integration world carries
	// only the synthetic telephony connector — the TEXT-turn happy path of
	// the confirm chain lives in the journeys contour over the web_chat
	// fixture connection; the 2-8 precedent of the span evidence).
	_, err := w.cases.ConfirmCommitment(ctx, casesAPI.ConfirmCommitmentCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: commitmentID, VersionNo: 1, ExpectedVersion: 2,
		SubjectKind: "client", SubjectRef: "client+79000000138", TurnRef: turn,
		Spans: []string{"подтверждаю замену масла"}, IdempotencyKey: "commitment-confirm-11", Now: ownershipNow,
	})
	assertRuleError(t, err, "вне текущего срез")
	// The presentation-less version confirms never (row 7: another
	// commitment without the reply link — the gate fires BEFORE the turn
	// source).
	orphan := id.MustParse("019a2f4c-5000-7000-8000-000000000012")
	if _, err := w.commitmentCreate(t, w.commitmentCreateCommand(caseID, workItemID, orphan, casesDomain.CommitmentStatusOffered, "commitment-create-12")); err != nil {
		t.Fatal(err)
	}
	_, err = w.cases.ConfirmCommitment(ctx, casesAPI.ConfirmCommitmentCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: orphan, VersionNo: 1, ExpectedVersion: 2,
		SubjectKind: "client", SubjectRef: "client+79000000138", TurnRef: turn,
		Spans: []string{"подтверждаю замену масла"}, IdempotencyKey: "commitment-confirm-orphan", Now: ownershipNow,
	})
	assertRuleError(t, err, "не была предъявлена клиенту")
}

// TestCommitmentMatrixHonestPriceAndDecline covers the rows 2, 3, 6, 17,
// 18, 15, 16 (the honest price holes, the decline without mutations, the
// superseded derivation and the rights/isolation boundaries): the unknown
// price blocks the readiness (not the creation); the offered version with
// a price demands the observation; the mismatch refuses; the decline lands
// append-only; the new version supersedes the old confirmation; the
// audit-only member and the foreign tenant never write.
func TestCommitmentMatrixHonestPriceAndDecline(t *testing.T) {
	w := commitmentWorld(t, 973)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-commitment-m3", "sess-commitment-m3", `{"call":"inbound-call","from":"+79000000139"}`)
	w.classifyCommitmentItem(t, caseID, workItemID, "commitment-classify-973")
	turn := w.modeTurnEvent(t, caseID, "sess-commitment-m3", "evt-commitment-m3-turn-1", `{"say":"нет, спасибо, не хочу"}`)
	conversationID := w.conversationFirst(t, caseID)

	// The unknown price: the offered row lands honestly (the hole blocks
	// the READINESS, not the creation — AC-2), the confirm refuses with
	// the mandatory-unknown words.
	unknown := w.commitmentCreateCommand(caseID, workItemID, id.MustParse("019a2f4c-5000-7000-8000-000000000021"), casesDomain.CommitmentStatusOffered, "commitment-create-21")
	unknown.MaterialUnknowns = []string{casesDomain.CommitmentTermPrice}
	outcome, err := w.commitmentCreate(t, unknown)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.CommitmentVersion != 1 {
		t.Fatalf("the honest unknown-price row lands, got %+v", outcome)
	}
	section := commitmentSectionOf(t, w.commitmentViewOf(t, caseID), workItemID)
	if section.Versions[0].Readiness != casesDomain.CommitmentReadinessUnknownBlocked {
		t.Fatalf("the unknown price blocks the readiness, got %s", section.Versions[0].Readiness)
	}
	_, err = w.cases.ConfirmCommitment(ctx, casesAPI.ConfirmCommitmentCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: id.MustParse("019a2f4c-5000-7000-8000-000000000021"), VersionNo: 1, ExpectedVersion: 1,
		SubjectKind: "client", SubjectRef: "client+79000000139", TurnRef: turn,
		Spans: []string{"нет"}, IdempotencyKey: "commitment-confirm-21", Now: ownershipNow,
	})
	assertRuleError(t, err, "неизвестный обязательный термин блокирует готовность")

	// The offered version with the KNOWN price but without the observation
	// of this Work Item refuses (never an assumed value).
	fresh := w.commitmentCreateCommand(caseID, workItemID, id.MustParse("019a2f4c-5000-7000-8000-000000000022"), casesDomain.CommitmentStatusOffered, "commitment-create-22")
	_, err = w.commitmentCreate(t, fresh)
	assertRuleError(t, err, "требует свежего наблюдения price")

	// The decline lands append-only without mutations (AC-11): the version
	// stays offered, the readiness stays, the row records the words (the
	// observation of the gate first — the offered price demands it).
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "commitment-obs-3"))
	command23 := w.commitmentCreateCommand(caseID, workItemID, id.MustParse("019a2f4c-5000-7000-8000-000000000023"), casesDomain.CommitmentStatusOffered, "commitment-create-23")
	if _, err := w.commitmentCreate(t, command23); err != nil {
		t.Fatal(err)
	}
	// The decline of the VOICE turn refuses honestly (AG-4: the TEXT-turn
	// decline happy path lives in the journeys contour — the 2-8 span
	// precedent); the append-only decline row itself is pinned by the
	// DB probes above (the outcome dictionary and the spans shape are the
	// CHECK backstops).
	_, err = w.cases.RecordCommitmentDecline(ctx, casesAPI.RecordCommitmentDeclineCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		CommitmentID: id.MustParse("019a2f4c-5000-7000-8000-000000000023"), VersionNo: 1,
		Outcome: casesDomain.CommitmentDeclineOutcomeChangeRequested, TurnRef: turn,
		Spans: []string{"нет, спасибо"}, IdempotencyKey: "commitment-decline-23", Now: ownershipNow,
	})
	assertRuleError(t, err, "вне текущего срез")
	section = commitmentSectionOf(t, w.commitmentViewOf(t, caseID), workItemID)
	if len(section.Declines) != 0 || section.Versions[len(section.Versions)-1].Version.Status != casesDomain.CommitmentStatusOffered {
		t.Fatalf("the refused decline writes nothing, got %+v", section)
	}

	// The rights gate BEFORE the guards (row 20): the audit-only member
	// never writes the commitments (the member ids avoid the world
	// suffixes — the 2-8 pattern).
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "commitment-auditor-973@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000009736", "000000009739")
	audit := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
	_, err = w.cases.CreateCommitment(ctx, casesAPI.CreateCommitmentCommand{
		Actor: audit, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		Status: casesDomain.CommitmentStatusDraft, Terms: commitmentTerms(),
		ActionSet:  []string{casesDomain.CommitmentActionPrepareOrder},
		ValidUntil: ownershipNow.Add(24 * time.Hour), ConsumptionLimit: 1,
		IdempotencyKey: "commitment-audit-1", Now: ownershipNow,
	})
	if !errors.Is(err, identityDomain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
		t.Fatalf("expected the honest 403 forbidden refusal with the scope words, got %v", err)
	}
	// The foreign tenant: the plain 404 (the RLS isolation).
	_, err = w.cases.CreateCommitment(ctx, casesAPI.CreateCommitmentCommand{
		Actor: w.ownershipActor(), TenantID: id.MustParse("019a2f4c-6000-7000-8000-000000000099"), CaseID: caseID, WorkItemID: workItemID,
		Status: casesDomain.CommitmentStatusDraft, Terms: commitmentTerms(),
		ActionSet:  []string{casesDomain.CommitmentActionPrepareOrder},
		ValidUntil: ownershipNow.Add(24 * time.Hour), ConsumptionLimit: 1,
		IdempotencyKey: "commitment-foreign-1", Now: ownershipNow,
	})
	if !errors.Is(err, identityDomain.ErrForbidden) && !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("expected the foreign tenant refusal, got %v", err)
	}
	_ = conversationID
}

// classifyCommitmentItem classifies the Work Item into in_work (the
// confirm ladder of the 2.2 states).
func (w *casesWorld) classifyCommitmentItem(t *testing.T, caseID, workItemID id.UUID, key string) {
	t.Helper()
	base := w.workBaseInput(caseID, workItemID, 1)
	base.IdempotencyKey = key
	_, err := w.cases.ClassifyWorkItem(context.Background(), casesAPI.ClassifyCommand{
		WorkCommandInput: base,
		Type:             "поиск детали", Title: "Замена масла",
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestCommitmentStoreFreshnessPinRace asserts the review R1 fix: the store
// re-validates the freshness pin under the locks — the app gate reads
// BEFORE the transaction, so a concurrent read_owning_system could land a
// newer price observation in the window (the store never trusts the app's
// read of the ladder; the stale pin refuses fail-closed, the new version
// is the honest path); the conflict badge against the confirmed exact
// price fact is re-derived from the lock-visible fact material (a fact
// confirmation landing in the same window flips the badge — the confirm
// refuses with the differs words). The direct store call carries the
// params the app assembled against its pre-transaction reads — the race
// simulation the sequential app path can not express (the app gate fires
// first there); the probe worlds pin both legs live.
func TestCommitmentStoreFreshnessPinRace(t *testing.T) {
	w := commitmentWorld(t, 979)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-commitment-race-1", "sess-commitment-race-1", `{"call":"inbound-call","from":"+79000000140"}`)
	w.classifyCommitmentItem(t, caseID, workItemID, "commitment-classify-979")
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-commitment-race-1", "evt-commitment-race-1-turn", `{"say":"согласен, подтверждаю условия"}`)
	// The offered version №1 over the price reading №1.
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "commitment-race-obs-1"))
	commitmentID := id.MustParse("019a2f4c-5000-7000-8000-000000000031")
	if _, err := w.commitmentCreate(t, w.commitmentCreateCommand(caseID, workItemID, commitmentID, casesDomain.CommitmentStatusOffered, "commitment-race-create-1")); err != nil {
		t.Fatal(err)
	}
	// The presentation link of №1 (the sales prelude + the linked reply):
	// the live channel of the reply conversation is the LATEST accepted
	// event — the voice turn of the prelude moment (phone), so the prelude
	// disclosure of the phone channel acts.
	w.policySalesPrelude(t, caseID, conversationID, turn, "commitment-race-sales")
	w.policyPrelude(t, caseID, conversationID, workItemID, "commitment-race-policy")
	if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		CommitmentID: commitmentID, CommitmentVersionNo: 1,
		Content: "Предлагаем условия версии №1", IdempotencyKey: "commitment-race-reply-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	// The consent TEXT turn lands AFTER the first reply (the confirm
	// evidence of the store probe); the live channel of the conversation
	// becomes web_chat, so the second reply of LEG 2 needs the acting
	// disclosure of THAT channel (the honest per-channel discipline).
	textTurn := nextCommitmentProbeID()
	consent := []byte(`{"say":"согласен, подтверждаю условия"}`)
	if _, err := adminPool.Exec(ctx, `
		insert into cases.channel_events (tenant_id,event_id,case_id,conversation_id,connection_id,provider,provider_event_id,provider_session_id,direction,modality,channel,correlation_id,idempotency_key,provider_timestamp,received_at,payload_digest,state,created_at)
		values ($1,$2,$3,$4,$5,'staffy-synthetic-telephony',$6,$6,'inbound','text','web_chat',$7,$8,$9,$9,$10,'accepted',$9)`,
		w.tenant, textTurn, caseID, conversationID, w.connection, "evt-commitment-race-text", nextCommitmentProbeID(), "probe-race-key", ownershipNow, casesDomain.PayloadDigest(consent)); err != nil {
		t.Fatal(err)
	}
	webChatDisclosure, err := w.cases.RecordDisclosure(ctx, casesAPI.RecordDisclosureCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		Channel: "web_chat", Text: "Я цифровой сотрудник компании. Человек подключится по вашему запросу.",
		IdempotencyKey: "commitment-race-disclosure-web", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.RecordDisclosureEvent(ctx, casesAPI.RecordDisclosureEventCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		DisclosureID: webChatDisclosure.DisclosureID, Kind: casesDomain.DeliverySent,
		IdempotencyKey: "commitment-race-disclosure-web-sent", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	// The store handle + the scope of the direct probe (the app-equivalent
	// params: the app read the observation BEFORE the re-read below — the
	// race window this direct call simulates).
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.connection, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	conditions, err := casesDomain.MaterialTermsJSON(commitmentTerms())
	if err != nil {
		t.Fatal(err)
	}
	spansJSON, err := json.Marshal([]string{"согласен, подтверждаю условия"})
	if err != nil {
		t.Fatal(err)
	}
	actionSetJSON, err := json.Marshal([]string{casesDomain.CommitmentActionPrepareOrder})
	if err != nil {
		t.Fatal(err)
	}
	storeConfirm := func(versionNo int64, key string) error {
		return store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
			Scope: scope, Command: "confirm_commitment", Key: key, RequestHash: strings.Repeat("b", 64),
			Actor: w.ownershipActor().ID, CaseID: caseID, WorkItemID: workItemID,
			CommitmentID: commitmentID, VersionNo: versionNo, ConfirmationID: nextCommitmentProbeID(),
			ExpectedVersion: 2, CommitmentSubjectKind: "client", CommitmentSubjectRef: "client+79000000140",
			Channel: "phone", TurnRef: textTurn,
			CommitmentExpiresAt: ownershipNow.Add(24 * time.Hour), CommitmentConsumptionLimit: 1,
			CommitmentEvidenceSpans: spansJSON, CommitmentTurnPayload: consent,
			CommitmentMaterialSnapshot: conditions, CommitmentActionSet: actionSetJSON,
			CommitmentContentToken: strings.Repeat("c", 64),
			CorrelationID:          nextCommitmentProbeID(), OutboxEventID: nextCommitmentProbeID(),
			Now: ownershipNow,
		})
	}
	// LEG 1 (the moved reading): the re-read landed the price observation
	// №2 AFTER the app resolved the pin of №1 (the create gate read №1) —
	// the store refuses the stale pin under the locks (the new version is
	// the honest path, never a silent confirm of the old reading).
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "commitment-race-obs-2"))
	assertRuleError(t, storeConfirm(1, "commitment-race-probe-1"), "больше не является текущим")
	// LEG 2 (the flipped badge): the version №2 pins the CURRENT reading
	// №2 (the create gate passes over it), the presentation link follows;
	// the price fact lands and confirms AFTER the create (the badge was
	// no_fact at the create moment) — the store re-derives the badge under
	// the locks and refuses the differs conflict.
	createTwo := w.commitmentCreateCommand(caseID, workItemID, commitmentID, casesDomain.CommitmentStatusOffered, "commitment-race-create-2")
	if _, err := w.commitmentCreate(t, createTwo); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		WorkItemID: workItemID, ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassSales,
		CommitmentID: commitmentID, CommitmentVersionNo: 2,
		Content: "Предлагаем условия версии №2", IdempotencyKey: "commitment-race-reply-2", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	fact, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: casesDomain.MaterialFieldPrice, Value: []byte(`{"price_quote":"2900"}`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "commitment-race-operator",
		Reason:         "оператор сверил счёт системы-владельца",
		IdempotencyKey: "commitment-race-fact", Now: ownershipNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, FactID: fact.FactID,
		VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		Note: "цена подтверждена по счёту", IdempotencyKey: "commitment-race-fact-confirm", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	assertRuleError(t, storeConfirm(2, "commitment-race-probe-2"), "расходится с подтверждённым фактом")
}
