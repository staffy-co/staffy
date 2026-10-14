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
	casesPort "github.com/artemiosu/staffy/internal/cases/port"
	casesStore "github.com/artemiosu/staffy/internal/cases/store"
	"github.com/artemiosu/staffy/internal/platform/id"
	"github.com/artemiosu/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5/pgconn"
)

// continuationSeedSeq keeps every seeded continuation-surface row
// globally unique across the whole suite (the commitment precedent: the
// ids of the direct admin inserts never collide with the command-path
// ids).
var continuationSeedSeq int64

// nextContinuationProbeID mints one globally unique probe id (the
// 019a2f4c-6000-8000 family — far from the command-path id space).
func nextContinuationProbeID() id.UUID {
	continuationSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 99000+continuationSeedSeq))
}

// The Story 2.10 step-02 probes: the migration 00010 invariants are
// pinned by the live probes immediately (the review R1 lesson of 2-7 —
// the invariant is never only a domain guard). Three probe families:
// the pg_class/RLS/grants discipline of the three append-only tables,
// the INSERT backstops (the dictionary CHECK words, the shape CHECKs,
// the token-hash UNIQUE, the one-revocation UNIQUE, the dictionary
// ALTERs of the commands, the outbox types and the fact channel word),
// and the store-level surface probes (the issue/revoke commands, the
// token-hash lookup under the RLS scope, the consumption accounting and
// the public submissions) — the honest rows land first, so every
// refusal below isolates exactly its dishonest part.

// TestContinuationTablesGrantsAndRLS asserts the append-only discipline
// of the continuation surface at the database level: RLS + FORCE on the
// three tables of 00010, INSERT/SELECT for staffy_app and no UPDATE,
// DELETE or TRUNCATE (the observation/objection/commitment precedent of
// 2.7/2.8/2.9).
func TestContinuationTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('continuation_links','continuation_link_uses','continuation_link_revocations')`)
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
	for _, table := range []string{"continuation_links", "continuation_link_uses", "continuation_link_revocations"} {
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

// continuationProbeRow carries the honest template of one continuation
// link row with the per-probe dishonest knobs.
type continuationProbeRow struct {
	linkID      id.UUID
	participant string
	epoch       int64
	purposes    string
	question    string
	explanation string
	ttl         int
	limit       int
	tokenHash   []byte
}

// honestContinuationRow is the honest template of the link (the
// dictionary words, the bounds, the 32-byte hash).
func honestContinuationRow(linkID id.UUID) continuationProbeRow {
	random, err := casesDomain.RandomContinuationSecret()
	if err != nil {
		panic(err)
	}
	hash, err := casesDomain.ContinuationTokenHash(random)
	if err != nil {
		panic(err)
	}
	return continuationProbeRow{
		linkID:      linkID,
		participant: "client+79000000137",
		epoch:       1,
		purposes:    `["fact_correction"]`,
		question:    "Какой верный VIN автомобиля?",
		explanation: "Переход нужен, чтобы исправить перенесённый факт без повторного разговора.",
		ttl:         30,
		limit:       1,
		tokenHash:   hash,
	}
}

// insertContinuationRow runs the direct INSERT of one link row over the
// live case + work item anchors (the admin role behind the RLS curtain
// — the DB-probe precedent of 2.7/2.8/2.9).
func (w *casesWorld) insertContinuationRow(t *testing.T, caseID, workItemID id.UUID, row continuationProbeRow) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.continuation_links (tenant_id, case_id, work_item_id, link_id, participant_ref,
                        owner_epoch, purposes, question, explanation, ttl_minutes, issued_at, expires_at,
                        consumption_limit, token_hash, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		w.tenant, caseID, workItemID, row.linkID, row.participant, row.epoch, row.purposes,
		row.question, row.explanation, row.ttl, ownershipNow, ownershipNow.Add(time.Duration(row.ttl)*time.Minute),
		row.limit, row.tokenHash, w.ownershipActor().ID, ownershipNow)
	return err
}

// TestContinuationDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00010 append-only surface: the dictionary CHECK words
// of the link row (the purposes, the use kinds), the shape CHECKs (the
// bounded texts, the ttl/limit bounds, the epoch, the 32-byte token
// hash, the expiry window arithmetic, the use material pair), the
// token-hash UNIQUE, the one-revocation-per-link UNIQUE, the nullable
// reply binding and the dictionary ALTERs (the two commands, the four
// outbox types, the fact channel word) — the honest shape is a database
// guarantee, not only a domain guard (the review R1 mutation lesson of
// 2.7: the invariants are pinned by the live probes immediately). The
// direct INSERTs run as the admin role behind the RLS curtain.
func TestContinuationDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 976)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-continuation-2-10-1", "sess-continuation-2-10-1", `{"call":"inbound-call","from":"+79000000137"}`)

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
	// linkProbe runs one dishonest link insert over the fresh honest
	// template (a fresh link id keeps the probes independent).
	linkProbe := func(mutate func(*continuationProbeRow)) error {
		row := honestContinuationRow(nextContinuationProbeID())
		mutate(&row)
		return w.insertContinuationRow(t, caseID, workItemID, row)
	}

	// The links: the honest row lands (the template is valid, so every
	// refusal below isolates exactly its dishonest part).
	if err := linkProbe(func(*continuationProbeRow) {}); err != nil {
		t.Fatalf("the honest continuation link row must land, got %v", err)
	}
	// The dictionary words and the bounds are the CHECK backstops.
	checkRefused("empty purposes", linkProbe(func(r *continuationProbeRow) { r.purposes = `[]` }))
	checkRefused("purpose outside the dictionary", linkProbe(func(r *continuationProbeRow) { r.purposes = `["media_upload"]` }))
	checkRefused("purposes not an array", linkProbe(func(r *continuationProbeRow) { r.purposes = `"fact_correction"` }))
	// The duplicate purposes discipline is the DOMAIN guard's honest 400
	// (the 2.9 precedent: the CHECK backstops the dictionary subset and
	// the bounds; the jsonb subset operator carries a duplicate array —
	// the store guard probe below refuses it under the locks).
	checkRefused("empty question", linkProbe(func(r *continuationProbeRow) { r.question = "" }))
	checkRefused("question over 500 runes", linkProbe(func(r *continuationProbeRow) { r.question = strings.Repeat("в", 501) }))
	checkRefused("empty explanation", linkProbe(func(r *continuationProbeRow) { r.explanation = "" }))
	checkRefused("explanation over 1000 runes", linkProbe(func(r *continuationProbeRow) { r.explanation = strings.Repeat("п", 1001) }))
	checkRefused("ttl below five minutes", linkProbe(func(r *continuationProbeRow) { r.ttl = 4 }))
	checkRefused("ttl over 120 minutes", linkProbe(func(r *continuationProbeRow) { r.ttl = 121 }))
	checkRefused("consumption limit below one", linkProbe(func(r *continuationProbeRow) { r.limit = 0 }))
	checkRefused("consumption limit over ten", linkProbe(func(r *continuationProbeRow) { r.limit = 11 }))
	checkRefused("owner epoch below one", linkProbe(func(r *continuationProbeRow) { r.epoch = 0 }))
	checkRefused("token hash under 32 bytes", linkProbe(func(r *continuationProbeRow) { r.tokenHash = make([]byte, 31) }))
	// The window CHECK: the honest template derives the expiry from the
	// ttl, so the dishonest row carries the expiry of a DIFFERENT ttl
	// through a dedicated insert.
	_, windowErr := adminPool.Exec(ctx, `
                insert into cases.continuation_links (tenant_id, case_id, work_item_id, link_id, participant_ref,
                        owner_epoch, purposes, question, explanation, ttl_minutes, issued_at, expires_at,
                        consumption_limit, token_hash, actor_id, created_at)
                values ($1,$2,$3,$4,'client+79000000137',1,'["fact_correction"]'::jsonb,'вопрос','объяснение',30,$5,$6,1,$7,$8,$9)`,
		w.tenant, caseID, workItemID, nextContinuationProbeID(), ownershipNow, ownershipNow.Add(29*time.Minute),
		honestContinuationRow(nextContinuationProbeID()).tokenHash, w.ownershipActor().ID, ownershipNow)
	checkRefused("the expiry outside the declared ttl window", windowErr)
	// The token hash is UNIQUE per tenant: the same random twice refuses
	// (the cryptographically unreachable collision — the backstop keeps
	// the material truth anyway).
	duplicateHashRow := honestContinuationRow(nextContinuationProbeID())
	if err := w.insertContinuationRow(t, caseID, workItemID, duplicateHashRow); err != nil {
		t.Fatalf("the honest duplicate-hash anchor row must land, got %v", err)
	}
	checkDuplicate("the duplicate token hash", linkProbe(func(r *continuationProbeRow) { r.tokenHash = duplicateHashRow.tokenHash }))

	// The anchor link of the use/revocation probes.
	anchor := honestContinuationRow(nextContinuationProbeID())
	if err := w.insertContinuationRow(t, caseID, workItemID, anchor); err != nil {
		t.Fatalf("the honest anchor link must land, got %v", err)
	}
	insertUse := func(useKind string, factID any, factVersion any) error {
		_, err := adminPool.Exec(ctx, `
                insert into cases.continuation_link_uses (tenant_id, case_id, work_item_id, link_id, use_id,
                        use_kind, fact_id, fact_version_no, submitted_at, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			w.tenant, caseID, workItemID, anchor.linkID, nextContinuationProbeID(), useKind, factID, factVersion, ownershipNow, ownershipNow)
		return err
	}
	// The uses: the honest rows land (the event without material, the
	// correction with the material pair).
	if err := insertUse("human_requested", nil, nil); err != nil {
		t.Fatalf("the honest human-request use row must land, got %v", err)
	}
	// The fact anchor of the material pair (the admin insert — the
	// confirmed-fact world of the store probes below builds the full
	// material).
	factID := nextContinuationProbeID()
	if _, err := adminPool.Exec(ctx, `
                insert into cases.facts (tenant_id, case_id, fact_id, material_key, created_at)
                values ($1,$2,$3,$4,$5)`, w.tenant, caseID, factID, "vin", ownershipNow); err != nil {
		t.Fatal(err)
	}
	if err := insertUse("fact_correction_submitted", factID, int64(2)); err != nil {
		t.Fatalf("the honest correction use row must land, got %v", err)
	}
	checkRefused("use kind outside the dictionary", insertUse("magic_use", nil, nil))
	checkRefused("the half material pair (fact without version)", insertUse("fact_correction_submitted", factID, nil))
	checkRefused("the half material pair (version without fact)", insertUse("fact_correction_submitted", nil, int64(1)))

	// The revocations: the honest row lands; the second revocation of
	// the SAME link refuses (the fail-closed one-revocation backstop);
	// the empty reason refuses.
	insertRevocation := func(reason string) error {
		_, err := adminPool.Exec(ctx, `
                insert into cases.continuation_link_revocations (tenant_id, case_id, work_item_id, link_id,
                        revocation_id, reason, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
			w.tenant, caseID, workItemID, anchor.linkID, nextContinuationProbeID(), reason, w.ownershipActor().ID, ownershipNow)
		return err
	}
	if err := insertRevocation("клиент перешёл в основной канал, ожидание прекращено"); err != nil {
		t.Fatalf("the honest revocation row must land, got %v", err)
	}
	checkDuplicate("the second revocation of the same link", insertRevocation("повторный отзыв"))
	checkRefused("the empty revocation reason", func() error {
		_, err := adminPool.Exec(ctx, `
                insert into cases.continuation_link_revocations (tenant_id, case_id, work_item_id, link_id,
                        revocation_id, reason, actor_id, created_at)
                values ($1,$2,$3,$4,$5,$6,$7,$8)`,
			w.tenant, caseID, workItemID, anchor.linkID, nextContinuationProbeID(), "", w.ownershipActor().ID, ownershipNow)
		return err
	}())

	// The presentation binding of the client replies: the nullable link
	// reference lands (the honest NULL of the legacy rows and the honest
	// binding of the sales reply — the column is one, no pair CHECK).
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
	insertReply := func(linkID any) error {
		_, err := adminPool.Exec(ctx, `
                insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id,
                        owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction,
                        employee_id, decision_no, profile_version, content, actor_id, created_at, link_id)
                values ($1,$2,$3,$4,'human',$5,1,'inbox',$6,'synthetic','web_chat','outbound',$7,1,1,$8,$9,$10,$11)`,
			w.tenant, caseID, conversationID, nextContinuationProbeID(),
			w.ownershipActor().ID, w.connection, w.employeeID,
			"перейдите по защищённой ссылке, чтобы исправить факт", w.ownershipActor().ID, ownershipNow, linkID)
		return err
	}
	if err := insertReply(anchor.linkID); err != nil {
		t.Fatalf("the honest link binding of the reply must land, got %v", err)
	}
	if err := insertReply(nil); err != nil {
		t.Fatalf("the honest legacy NULL binding must land, got %v", err)
	}

	// The two command words of 00010 join the idempotency dictionary.
	insertCommand := func(command string) error {
		_, err := adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,$3,$4,$5,'{"ok":true}'::jsonb,$6)`,
			w.tenant, w.ownershipActor().ID, command,
			fmt.Sprintf("continuation-probe-%d", continuationSeedSeq), strings.Repeat("c", 64), ownershipNow)
		return err
	}
	if err := insertCommand("issue_continuation_link"); err != nil {
		t.Fatalf("the issue_continuation_link command word must land, got %v", err)
	}
	if err := insertCommand("revoke_continuation_link"); err != nil {
		t.Fatalf("the revoke_continuation_link command word must land, got %v", err)
	}
	checkRefused("the command outside the dictionary", insertCommand("magic_command"))

	// The four outbox event types of 00010 are the dictionary words: the
	// continuation events land, the magic type refuses (the envelope
	// carries the metadata only — the token and the texts never enter).
	insertOutbox := func(eventType string) error {
		_, err := adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id, event_id, event_type, schema_version, aggregate_id,
                        source_connection_id, correlation_id, occurred_at, payload, dedup_key, created_at)
                values ($1,$2,$3,1,$4,$5,$6,$7,'{"kind":"probe"}'::jsonb,$8,$9)`,
			w.tenant, nextContinuationProbeID(), eventType, caseID, w.connection, nextContinuationProbeID(),
			ownershipNow, fmt.Sprintf("continuation-probe-%d", continuationSeedSeq), ownershipNow)
		return err
	}
	for _, eventType := range []string{
		"case.continuation_link_issued", "case.continuation_submitted",
		"case.continuation_human_requested", "case.continuation_returned",
	} {
		if err := insertOutbox(eventType); err != nil {
			t.Fatalf("the %s event type must land, got %v", eventType, err)
		}
	}
	checkRefused("the event type outside the dictionary", insertOutbox("case.magic_event"))

	// The protected-continuation word joins the fact channel dictionary:
	// the client-submitted correction version lands with the new
	// pseudo-channel; the pre-existing words stay legal (the control
	// branch — the 2.2 rows never break).
	insertFactVersion := func(channel string) error {
		_, err := adminPool.Exec(ctx, `
                insert into cases.fact_versions (tenant_id, case_id, fact_id, version_no, work_item_id, material_key,
                        value, value_digest, source_type, source_id, channel, actor_id, observed_at, received_at, created_at)
                values ($1,$2,$3,1,$4,'vin','"probe"'::jsonb,$5,'operator','probe-command-key-0001',$6,$7,$8,$9,$10)`,
			w.tenant, caseID, factID, workItemID, strings.Repeat("d", 64), channel, w.ownershipActor().ID,
			ownershipNow, ownershipNow, ownershipNow)
		return err
	}
	if err := insertFactVersion("protected_continuation"); err != nil {
		t.Fatalf("the protected_continuation channel word must land (the dictionary extension), got %v", err)
	}
	if _, err := adminPool.Exec(ctx, `
                insert into cases.fact_versions (tenant_id, case_id, fact_id, version_no, work_item_id, material_key,
                        value, value_digest, source_type, source_id, channel, actor_id, observed_at, received_at, created_at)
                values ($1,$2,$3,2,$4,'vin','"probe2"'::jsonb,$5,'operator','probe-command-key-0002','operator',$6,$7,$8,$9)`,
		w.tenant, caseID, factID, workItemID, strings.Repeat("e", 64), w.ownershipActor().ID,
		ownershipNow, ownershipNow, ownershipNow); err != nil {
		t.Fatalf("the pre-existing operator channel word must stay legal (the control branch), got %v", err)
	}
	checkRefused("the channel outside the dictionary", insertFactVersion("magic_channel"))
}

// continuationForeignTenant is the tenant of the RLS-lookup probe: a
// uuid that never carries a world (the wrong-tenant scope candidate of
// the AD-10 honesty — the same token hash finds no row under the
// foreign tenant scope, the generic refusal without disclosure).
var continuationForeignTenant = id.MustParse("019a2f4c-6000-7000-8000-000000000099")

// TestContinuationStoreIssueLookupSubmit pins the store-level surface of
// the slice: the issue command writes the link with the outbox event
// (the metadata only), the token-hash lookup derives the state
// read-time, the RLS scope keeps the foreign tenant blind (the same
// hash finds no row), the guard probes refuse the dishonest issuance
// honestly, the command-key backstop keeps the raced repeat the honest
// conflict, the human event consumes nothing, the correction writes the
// unverified new version with the protected-continuation provenance and
// consumes the limit (the confirmed-fact gate: after the correction the
// fact leaves the confirmed set — the second correction of the SAME
// fact refuses honestly; the second link fact consumes to the limit),
// the revoke blocks the surface read-time and the second revoke of the
// same link refuses, and the panel read carries the links, the uses,
// the revocations and the delivered binding.
func TestContinuationStoreIssueLookupSubmit(t *testing.T) {
	w := newCasesWorld(t, 977)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-continuation-store-1", "sess-continuation-store-1", `{"call":"inbound-call","from":"+79000000137"}`)
	store := casesStore.NewPostgres(runtimePool)
	// The command-probe scope: the ACTOR is the principal (the
	// command_records RLS policy demands actor == principal on the
	// successful insert — the discipline the app's own scope carries).
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	// The public-surface scope: an INERT principal (the real semantics of
	// the token-carried surface — the public writes never read the
	// principal; the rows carry the resolved link id as their actor).
	publicScope, err := tenancy.New(nextContinuationProbeID(), w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	// The honest issuance material: the opaque token halves (the random
	// never enters the database — only its SHA-256), the purposes JSON,
	// the outcome of the command record (the replay answer carries the
	// link id and the mask — the token NEVER enters the outcome either).
	random, err := casesDomain.RandomContinuationSecret()
	if err != nil {
		t.Fatal(err)
	}
	tokenHash, err := casesDomain.ContinuationTokenHash(random)
	if err != nil {
		t.Fatal(err)
	}
	purposesJSON, err := json.Marshal([]string{casesDomain.ContinuationPurposeFactCorrection})
	if err != nil {
		t.Fatal(err)
	}
	outcomeJSON, err := json.Marshal(map[string]any{"link_id": "issued", "token": "выдан"})
	if err != nil {
		t.Fatal(err)
	}
	linkID := nextContinuationProbeID()
	issueParams := func() casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: scope, Command: "issue_continuation_link", Key: "continuation-issue-store-1",
			RequestHash: strings.Repeat("a", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, WorkItemID: workItemID, LinkID: linkID,
			ContinuationPurposes:       purposesJSON,
			ContinuationQuestion:       "Какой верный VIN автомобиля?",
			ContinuationExplanation:    "Переход нужен, чтобы исправить перенесённый факт без повторного разговора.",
			ContinuationParticipantRef: "client+79000000137",
			ContinuationTTLMinutes:     30, ContinuationLimit: 2,
			ContinuationTokenHash: tokenHash,
			CorrelationID:         nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
			Outcome: outcomeJSON, Now: ownershipNow,
		}
	}
	if err := store.ExecuteWorkCommand(ctx, issueParams()); err != nil {
		t.Fatalf("the honest issuance must land, got %v", err)
	}

	// The token-hash lookup: the derived state is valid, no uses, the
	// current owner epoch matches the pin, the row carries the material
	// of the surface.
	lookup, err := store.LookupContinuationByTokenHash(ctx, scope, tokenHash, ownershipNow)
	if err != nil {
		t.Fatal(err)
	}
	if lookup.State != casesDomain.ContinuationStateValid {
		t.Fatalf("expected the valid state, got %s", lookup.State)
	}
	if lookup.Link.LinkID != linkID || len(lookup.Uses) != 0 || lookup.OwnerEpochCurrent != lookup.Link.OwnerEpoch {
		t.Fatalf("the lookup material is wrong: %+v", lookup)
	}
	if lookup.Link.Question != "Какой верный VIN автомобиля?" || lookup.Link.ConsumptionLimit != 2 {
		t.Fatalf("the link row material is wrong: %+v", lookup.Link)
	}

	// The RLS-lookup probe (the AD-10 honesty): the same token hash under
	// the FOREIGN tenant scope finds no row — the generic refusal
	// without disclosure (the wrong-tenant scope candidate never reads
	// any case material).
	foreignScope, err := tenancy.New(nextContinuationProbeID(), continuationForeignTenant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.LookupContinuationByTokenHash(ctx, foreignScope, tokenHash, ownershipNow); !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the foreign-tenant lookup must refuse with ErrNotFound, got %v", err)
	}
	// The unknown hash refuses with the same word (the lookup layer).
	if _, err = store.LookupContinuationByTokenHash(ctx, scope, []byte(strings.Repeat("x", 32)), ownershipNow); !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the unknown-hash lookup must refuse with ErrNotFound, got %v", err)
	}

	// The guard probes of the issuance (the honest 400 words of the
	// domain guards re-validated under the locks).
	guardProbe := func(mutate func(*casesPort.WorkCommandParams)) error {
		params := issueParams()
		params.Key = fmt.Sprintf("continuation-issue-guard-%d", continuationSeedSeq)
		params.LinkID = nextContinuationProbeID()
		random, err := casesDomain.RandomContinuationSecret()
		if err != nil {
			t.Fatal(err)
		}
		hash, err := casesDomain.ContinuationTokenHash(random)
		if err != nil {
			t.Fatal(err)
		}
		params.ContinuationTokenHash = hash
		mutate(&params)
		return store.ExecuteWorkCommand(ctx, params)
	}
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationQuestion = "" }), "открытый вопрос")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationExplanation = "" }), "объяснение перехода")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationTTLMinutes = 4 }), "5..120")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationLimit = 11 }), "1..10")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationPurposes = []byte(`["media_upload"]`) }), "не входит в словарь")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) {
		p.ContinuationPurposes = []byte(`["fact_correction","fact_correction"]`)
	}), "повторы")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationPurposes = []byte(`[]`) }), "не может быть пустым")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationParticipantRef = "" }), "1..128")
	assertRuleError(t, guardProbe(func(p *casesPort.WorkCommandParams) { p.ContinuationTokenHash = make([]byte, 31) }), "32 байта")

	// The raced repeat of the identical material (the same link id under
	// the same command key): the loser-of-the-race signal — the honest
	// stale refusal (the app retry replays the stored outcome through the
	// command record; the objection/commitment precedent).
	if err := store.ExecuteWorkCommand(ctx, issueParams()); !errors.Is(err, casesDomain.ErrStaleVersion) {
		t.Fatalf("the raced identical issuance must refuse with the stale signal, got %v", err)
	}
	// The same command key with ANOTHER link material: the exec succeeds
	// (the fresh link row), the command record insert conflicts and the
	// whole transaction rolls back — the honest material conflict.
	conflictParams := issueParams()
	conflictParams.LinkID = nextContinuationProbeID()
	conflictRandom, conflictErr := casesDomain.RandomContinuationSecret()
	if conflictErr != nil {
		t.Fatal(conflictErr)
	}
	conflictHash, conflictErr := casesDomain.ContinuationTokenHash(conflictRandom)
	if conflictErr != nil {
		t.Fatal(conflictErr)
	}
	conflictParams.ContinuationTokenHash = conflictHash
	if err := store.ExecuteWorkCommand(ctx, conflictParams); !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another link material must conflict, got %v", err)
	}
	// The rollback undid the conflicting link: the case still carries
	// exactly one issued link.
	material, err := store.ReadContinuations(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(material.Links) != 1 {
		t.Fatalf("the conflicting issuance must roll back completely, got %d links", len(material.Links))
	}

	// The human event: the use row lands, nothing is consumed, no work
	// item material is touched.
	if err := store.SubmitContinuationEvent(ctx, casesPort.ContinuationEventParams{
		Scope: publicScope, TokenHash: tokenHash, UseKind: casesDomain.ContinuationUseHumanRequested,
		UseID: nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		CorrelationID: nextContinuationProbeID(), Now: ownershipNow.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	lookup, err = store.LookupContinuationByTokenHash(ctx, scope, tokenHash, ownershipNow)
	if err != nil {
		t.Fatal(err)
	}
	if lookup.State != casesDomain.ContinuationStateValid || len(lookup.Uses) != 1 {
		t.Fatalf("the human event must not consume: state=%s uses=%d", lookup.State, len(lookup.Uses))
	}

	// The confirmed facts of the correction world (the admin inserts —
	// the DB-probe precedent): two confirmed facts of the work item (the
	// limit of 2 carries two submissions of two DIFFERENT facts; after a
	// correction the fact leaves the confirmed set — the honest gate).
	factA := nextContinuationProbeID()
	factB := nextContinuationProbeID()
	seedConfirmedFact := func(factID id.UUID, materialKey, rawValue string) {
		t.Helper()
		value, digest, valueErr := casesDomain.FactValue([]byte(rawValue))
		if valueErr != nil {
			t.Fatal(valueErr)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into cases.facts (tenant_id, case_id, fact_id, material_key, created_at)
                        values ($1,$2,$3,$4,$5)`, w.tenant, caseID, factID, materialKey, ownershipNow); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into cases.fact_versions (tenant_id, case_id, fact_id, version_no, work_item_id, material_key,
                                value, value_digest, source_type, source_id, channel, actor_id, observed_at, received_at, created_at)
                        values ($1,$2,$3,1,$4,$5,$6::jsonb,$7,'operator',$8,'operator',$9,$10,$11,$12)`,
			w.tenant, caseID, factID, workItemID, materialKey, string(value), digest,
			fmt.Sprintf("probe-command-key-%d", continuationSeedSeq), w.ownershipActor().ID,
			ownershipNow, ownershipNow, ownershipNow); err != nil {
			t.Fatal(err)
		}
		if _, err := adminPool.Exec(ctx, `
                        insert into cases.fact_confirmations (tenant_id, fact_id, confirmation_id, version_no, confirmer_type,
                                evidence_event_id, note, actor_id, created_at)
                        values ($1,$2,$3,1,'operator_relay',null,$4,$5,$6)`,
			w.tenant, factID, nextContinuationProbeID(), "подтверждено перечиткой в разговоре",
			w.ownershipActor().ID, ownershipNow); err != nil {
			t.Fatal(err)
		}
	}
	seedConfirmedFact(factA, "vin", `"VIN0000000000001"`)
	seedConfirmedFact(factB, "mileage", `"120000"`)

	// The first correction: the new unverified version №2 with the
	// protected-continuation provenance (the channel word, the link
	// source ref, the capability as the actor), the use row consumes.
	versionNo, err := store.SubmitContinuationCorrection(ctx, casesPort.ContinuationCorrectionParams{
		Scope: publicScope, TokenHash: tokenHash, FactID: factA, Value: []byte(`"VIN0000000000009"`),
		UseID: nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		CorrelationID: nextContinuationProbeID(), Now: ownershipNow.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if versionNo != 2 {
		t.Fatalf("the correction must write the version 2, got %d", versionNo)
	}
	var channel, sourceType, sourceID, actorID string
	if err := adminPool.QueryRow(ctx, `
                select channel, source_type, source_id, actor_id::text from cases.fact_versions
                where tenant_id=$1 and fact_id=$2 and version_no=2`,
		w.tenant, factA).Scan(&channel, &sourceType, &sourceID, &actorID); err != nil {
		t.Fatal(err)
	}
	if channel != casesDomain.FactChannelProtectedContinuation || sourceType != casesDomain.ContinuationCorrectionSourceType ||
		sourceID != linkID.String() || actorID != linkID.String() {
		t.Fatalf("the correction provenance is wrong: channel=%s source=%s/%s actor=%s", channel, sourceType, sourceID, actorID)
	}
	lookup, err = store.LookupContinuationByTokenHash(ctx, scope, tokenHash, ownershipNow)
	if err != nil {
		t.Fatal(err)
	}
	if lookup.State != casesDomain.ContinuationStateValid || len(lookup.Uses) != 2 {
		t.Fatalf("one correction of two must keep the link valid: state=%s uses=%d", lookup.State, len(lookup.Uses))
	}

	// The same-fact second correction refuses honestly: the corrected
	// fact left the confirmed set (the latest version differs from the
	// last confirmed one — the operator resolves it through the штатный
	// 2.2 command; the surface offers the confirmed facts only).
	_, err = store.SubmitContinuationCorrection(ctx, casesPort.ContinuationCorrectionParams{
		Scope: publicScope, TokenHash: tokenHash, FactID: factA, Value: []byte(`"VIN0000000000010"`),
		UseID: nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		CorrelationID: nextContinuationProbeID(), Now: ownershipNow.Add(3 * time.Minute),
	})
	assertRuleError(t, err, "подтверждённый факт")

	// The second correction of the SECOND fact consumes the link to the
	// limit: the state derives consumed (the read-time truth — a dead
	// link never resurrects).
	versionNo, err = store.SubmitContinuationCorrection(ctx, casesPort.ContinuationCorrectionParams{
		Scope: publicScope, TokenHash: tokenHash, FactID: factB, Value: []byte(`"125000"`),
		UseID: nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		CorrelationID: nextContinuationProbeID(), Now: ownershipNow.Add(4 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if versionNo != 2 {
		t.Fatalf("the second fact correction must write the version 2, got %d", versionNo)
	}
	lookup, err = store.LookupContinuationByTokenHash(ctx, scope, tokenHash, ownershipNow)
	if err != nil {
		t.Fatal(err)
	}
	if lookup.State != casesDomain.ContinuationStateConsumed {
		t.Fatalf("two accepted corrections of the limit 2 must consume the link, got %s", lookup.State)
	}

	// The third correction (any fact) refuses: the consumed link is the
	// terminal «материал отправлен» state (the PRG double-click is
	// honestly bounded by the limit).
	_, err = store.SubmitContinuationCorrection(ctx, casesPort.ContinuationCorrectionParams{
		Scope: publicScope, TokenHash: tokenHash, FactID: factB, Value: []byte(`"130000"`),
		UseID: nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		CorrelationID: nextContinuationProbeID(), Now: ownershipNow.Add(5 * time.Minute),
	})
	assertRuleError(t, err, "недоступна для действий")

	// The events of the consumed link refuse too (the surface is one
	// lifecycle: the consumed word blocks every action).
	err = store.SubmitContinuationEvent(ctx, casesPort.ContinuationEventParams{
		Scope: publicScope, TokenHash: tokenHash, UseKind: casesDomain.ContinuationUseReturnedToConversation,
		UseID: nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		CorrelationID: nextContinuationProbeID(), Now: ownershipNow.Add(6 * time.Minute),
	})
	assertRuleError(t, err, "недоступна для действий")

	// The revocation: the second link, issued and revoked through the
	// command surface — the state derives revoked read-time, the second
	// revoke of the same link refuses (the append-only history says the
	// expectation ended once).
	secondRandom, err := casesDomain.RandomContinuationSecret()
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := casesDomain.ContinuationTokenHash(secondRandom)
	if err != nil {
		t.Fatal(err)
	}
	secondLink := nextContinuationProbeID()
	secondOutcome, err := json.Marshal(map[string]any{"link_id": "issued", "token": "выдан"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
		Scope: scope, Command: "issue_continuation_link", Key: "continuation-issue-store-2",
		RequestHash: strings.Repeat("b", 64), Actor: w.ownershipActor().ID,
		CaseID: caseID, WorkItemID: workItemID, LinkID: secondLink,
		ContinuationPurposes:       purposesJSON,
		ContinuationQuestion:       "Проверьте марку автомобиля",
		ContinuationExplanation:    "Переход для сверки одного факта.",
		ContinuationParticipantRef: "client+79000000137",
		ContinuationTTLMinutes:     15, ContinuationLimit: 1,
		ContinuationTokenHash: secondHash,
		CorrelationID:         nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		Outcome: secondOutcome, Now: ownershipNow.Add(7 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	revoke := func() error {
		return store.ExecuteWorkCommand(ctx, casesPort.WorkCommandParams{
			Scope: scope, Command: "revoke_continuation_link", Key: "continuation-revoke-store-1",
			RequestHash: strings.Repeat("c", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, WorkItemID: workItemID, LinkID: secondLink,
			RevocationID:       nextContinuationProbeID(),
			ContinuationReason: "клиент передумал переходить по ссылке, ожидание прекращено",
			CorrelationID:      nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
			Outcome: secondOutcome, Now: ownershipNow.Add(8 * time.Minute),
		})
	}
	if err := revoke(); err != nil {
		t.Fatalf("the honest revocation must land, got %v", err)
	}
	secondLookup, err := store.LookupContinuationByTokenHash(ctx, scope, secondHash, ownershipNow.Add(8*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if secondLookup.State != casesDomain.ContinuationStateRevoked || !secondLookup.Revoked {
		t.Fatalf("the revoked link must derive the revoked state, got %s", secondLookup.State)
	}
	// The revoked surface blocks the actions (the same honest refusal).
	err = store.SubmitContinuationEvent(ctx, casesPort.ContinuationEventParams{
		Scope: publicScope, TokenHash: secondHash, UseKind: casesDomain.ContinuationUseHumanRequested,
		UseID: nextContinuationProbeID(), OutboxEventID: nextContinuationProbeID(),
		CorrelationID: nextContinuationProbeID(), Now: ownershipNow.Add(9 * time.Minute),
	})
	assertRuleError(t, err, "недоступна для действий")
	// The second revoke of the same link refuses (the one-revocation
	// discipline).
	assertRuleError(t, revoke(), "уже отозвана")

	// The panel read: the links, the uses, the revocations and the
	// delivered binding (the reply that wove the URL — the FR-16
	// «доставлена»).
	material, err = store.ReadContinuations(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(material.Links) != 2 || len(material.Uses) != 3 || len(material.Revocations) != 1 {
		t.Fatalf("the panel material is wrong: links=%d uses=%d revocations=%d",
			len(material.Links), len(material.Uses), len(material.Revocations))
	}
	if material.Delivered[linkID] || material.Delivered[secondLink] {
		t.Fatalf("no reply carried the links yet — the delivered set must be empty, got %+v", material.Delivered)
	}
	if _, err := adminPool.Exec(ctx, `
                insert into cases.client_replies (tenant_id, case_id, conversation_id, reply_id,
                        owner_kind, owner_ref, epoch, writable_surface, connection_id, provider, channel, direction,
                        employee_id, decision_no, profile_version, content, actor_id, created_at, link_id)
                values ($1,$2,(select conversation_id from cases.conversations where tenant_id=$1 and case_id=$2 order by conversation_no limit 1),
                        $3,'human',$4,1,'inbox',$5,'synthetic','web_chat','outbound',$6,1,1,$7,$8,$9,$10)`,
		w.tenant, caseID, nextContinuationProbeID(), w.ownershipActor().ID, w.connection, w.employeeID,
		"перейдите по защищённой ссылке и исправьте VIN", w.ownershipActor().ID, ownershipNow.Add(10*time.Minute), secondLink); err != nil {
		t.Fatal(err)
	}
	material, err = store.ReadContinuations(ctx, scope, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if !material.Delivered[secondLink] || material.Delivered[linkID] {
		t.Fatalf("the delivered binding must carry the second link only, got %+v", material.Delivered)
	}

	// The outbox honesty: every continuation envelope of the case carries
	// the identifiers and the dictionary words only — the question text,
	// the explanation text and the corrected value never enter (the
	// «кандидат не покидает строку» precedent; the token never travels at
	// all).
	rows, err := adminPool.Query(ctx, `
                select event_type, payload::text from cases.outbox_events
                where tenant_id=$1 and aggregate_id=$2 and event_type like 'case.continuation%'
                order by created_at`, w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	envelopes := map[string]int{}
	for rows.Next() {
		var eventType, payload string
		if err := rows.Scan(&eventType, &payload); err != nil {
			t.Fatal(err)
		}
		envelopes[eventType]++
		for _, forbidden := range []string{"Какой верный VIN", "Переход нужен", "VIN0000000000009", "125000"} {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("the %s envelope must never carry the material %q: %s", eventType, forbidden, payload)
			}
		}
		if strings.Contains(strings.ToLower(payload), "token") {
			// the outcome mask word «выдан» is the command record, not
			// the envelope; the envelope never carries a token field
			if strings.Contains(payload, "token_hash") || strings.Contains(payload, "\"token\"") {
				t.Fatalf("the %s envelope must never carry the token: %s", eventType, payload)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if envelopes["case.continuation_link_issued"] != 2 || envelopes["case.continuation_submitted"] != 2 ||
		envelopes["case.continuation_human_requested"] != 1 || envelopes["case.continuation_returned"] != 0 {
		t.Fatalf("the continuation envelopes are wrong: %+v", envelopes)
	}
}
