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

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	casesStore "github.com/staffy-co/staffy/internal/cases/store"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
	"github.com/jackc/pgx/v5/pgconn"
)

// summarySeedSeq keeps every seeded summary-surface row globally unique
// across the whole suite (the degradation and handoff precedents).
var summarySeedSeq int64

// nextSummaryProbeID mints one globally unique probe id.
func nextSummaryProbeID() id.UUID {
	summarySeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-7000-7000-8000-%012d", 99300+summarySeedSeq))
}

// The Story 2.14 step-02 probes: the migration 00014 invariants are
// pinned by the live probes immediately (the review R1 lesson of 2.7 —
// the invariant is never only a domain guard). Three probe families:
// the pg_class/RLS/grants discipline of the append-only summary table,
// the INSERT backstops (the kind dictionary, the text bounds, the
// claims material check — the linked-or-disputed discipline of every
// element, the array shape, the summary number UNIQUE, the case FK, the
// watermark bounds, the command/outbox dictionary ALTERs), and the
// store-level probes (the ownership gate BEFORE the material guards,
// the ref guards of the claims, the domain guard under the locks, the
// SERVER-truth watermark, the monotonic summary numbers, the key
// conflict of the same key with another material, the identifiers-only
// outbox envelope, the ONE-transaction context snapshot with the
// read-time staleness derivation, and the perimeter absence in the
// fact channels).

// TestSummariesTablesGrantsAndRLS asserts the append-only discipline of
// the summary surface at the database level: RLS + FORCE, INSERT/SELECT
// for staffy_app and no UPDATE, DELETE or TRUNCATE (the degradation and
// handoff precedents).
func TestSummariesTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('case_summaries')`)
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
	if state, ok := seen["case_summaries"]; !ok || !state[0] || !state[1] {
		t.Fatalf("case_summaries must carry RLS + FORCE (the append-only discipline), got %+v", state)
	}
	for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
		var allowed bool
		if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases.case_summaries", grant).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if (grant == "INSERT" || grant == "SELECT") != allowed {
			t.Fatalf("%s on cases.case_summaries: allowed=%v, want %v", grant, allowed, grant == "INSERT" || grant == "SELECT")
		}
	}
}

// summaryProbeRow carries the honest template of one summary row with
// the per-probe dishonest knobs.
type summaryProbeRow struct {
	summaryNo int64
	kind      string
	text      string
	claims    string
}

// insertSummaryRow runs the direct INSERT of one summary row over the
// live case anchor (the admin role behind the RLS curtain — the
// DB-probe precedent of 2.7..2.13).
func (w *casesWorld) insertSummaryRow(t *testing.T, caseID id.UUID, row summaryProbeRow) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.case_summaries (tenant_id, case_id, summary_no, kind, actor_id, text, claims,
                        event_count, fact_version_count, owner_epoch, mode_version, created_at)
                values ($1,$2,$3,$4,$5,$6,$7::jsonb,0,0,1,1,$8)`,
		w.tenant, caseID, row.summaryNo, row.kind, w.ownershipActor().ID,
		row.text, row.claims, ownershipNow)
	return err
}

// honestSummaryClaims renders the honest claims JSON of the probes: one
// linked claim (the ref-type word with the ref id) and one honestly
// disputed claim (the byte-exact span discipline belongs to the domain
// guard — the database check is the material shape).
func honestSummaryClaims() string {
	return `[{"ref_type":"fact_version","ref_id":"` + id.MustParse("019a2f4c-7000-7000-8000-000000000001").String() + `","disputed":false,"text":"Клиент назвал адрес."},{"disputed":true,"text":"Срок доставки не назван."}]`
}

// manyClaims renders the claims JSON of the given length (the array
// bound probes — every element honest).
func manyClaims(count int) string {
	elements := make([]string, count)
	for i := range elements {
		elements[i] = `{"disputed":true,"text":"x"}`
	}
	return "[" + strings.Join(elements, ",") + "]"
}

// TestSummariesDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00014 surface: the kind dictionary (the SINGLE word
// of the slice — the operator summaries are Never), the text bounds,
// the claims array shape (1..32 elements, every element an object), the
// MATERAL linked-or-disputed discipline of every element (the ref-type
// word of the fixed dictionary WITH a non-empty ref id, or the honest
// disputed mark), the summary number monotonicity UNIQUE, the case FK,
// the watermark bounds, and the command/outbox dictionary ALTERs — the
// honest shape is a database guarantee, not only a domain guard.
func TestSummariesDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 991)
	ctx := context.Background()
	caseID, _ := w.acceptedCase(t, "evt-summary-2-14-1", "sess-summary-2-14-1", `{"call":"inbound-call","from":"+79000000231"}`)

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
	honestRow := func(no int64) summaryProbeRow {
		return summaryProbeRow{
			summaryNo: no, kind: casesDomain.SummaryKindAI,
			text: "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1.", claims: honestSummaryClaims(),
		}
	}

	// The honest summary row lands (the single kind word, the bound text,
	// the linked-and-disputed claims, the watermark columns).
	if err := w.insertSummaryRow(t, caseID, honestRow(1)); err != nil {
		t.Fatalf("the honest summary must land, got %v", err)
	}

	// The kind dictionary and the text bounds.
	checkRefused("foreign kind word", w.insertSummaryRow(t, caseID, func() summaryProbeRow { r := honestRow(2); r.kind = "operator_notes"; return r }()))
	checkRefused("empty text", w.insertSummaryRow(t, caseID, func() summaryProbeRow { r := honestRow(2); r.text = ""; return r }()))
	checkRefused("the 1001-rune text", w.insertSummaryRow(t, caseID, func() summaryProbeRow { r := honestRow(2); r.text = strings.Repeat("а", 1001); return r }()))

	// The claims array shape: 1..32 elements, an array of objects.
	checkRefused("empty claims array", w.insertSummaryRow(t, caseID, func() summaryProbeRow { r := honestRow(2); r.claims = "[]"; return r }()))
	checkRefused("the 33-claim array", w.insertSummaryRow(t, caseID, func() summaryProbeRow { r := honestRow(2); r.claims = manyClaims(33); return r }()))
	checkRefused("non-array claims", w.insertSummaryRow(t, caseID, func() summaryProbeRow { r := honestRow(2); r.claims = `"claim"`; return r }()))
	checkRefused("non-object element", w.insertSummaryRow(t, caseID, func() summaryProbeRow { r := honestRow(2); r.claims = `["claim"]`; return r }()))

	// The MATERIAL linked-or-disputed discipline of every element: a
	// claim either links (the ref-type word of the dictionary WITH a
	// non-empty ref id) or carries the honest disputed mark.
	checkRefused("unlinked undisputed claim", w.insertSummaryRow(t, caseID, func() summaryProbeRow {
		r := honestRow(2)
		r.claims = `[{"text":"Срок доставки не назван."}]`
		return r
	}()))
	checkRefused("foreign ref-type word", w.insertSummaryRow(t, caseID, func() summaryProbeRow {
		r := honestRow(2)
		r.claims = `[{"ref_type":"mandate","ref_id":"` + id.MustParse("019a2f4c-7000-7000-8000-000000000001").String() + `","text":"x"}]`
		return r
	}()))
	checkRefused("empty ref id", w.insertSummaryRow(t, caseID, func() summaryProbeRow {
		r := honestRow(2)
		r.claims = `[{"ref_type":"fact_version","ref_id":"","text":"x"}]`
		return r
	}()))
	checkRefused("missing ref id", w.insertSummaryRow(t, caseID, func() summaryProbeRow {
		r := honestRow(2)
		r.claims = `[{"ref_type":"fact_version","text":"x"}]`
		return r
	}()))

	// The honest disputed claim without a ref lands (the honest boundary
	// of an unsourced assertion — the OR discipline).
	if err := w.insertSummaryRow(t, caseID, func() summaryProbeRow {
		r := honestRow(2)
		r.claims = `[{"disputed":true,"text":"Срок доставки не назван."}]`
		return r
	}()); err != nil {
		t.Fatalf("the honestly disputed claim must land, got %v", err)
	}
	// The linked AND disputed claim lands (the OR discipline: the mark
	// never forbids the link).
	if err := w.insertSummaryRow(t, caseID, func() summaryProbeRow {
		r := honestRow(3)
		r.claims = `[{"ref_type":"observation","ref_id":"` + id.MustParse("019a2f4c-7000-7000-8000-000000000002").String() + `","disputed":true,"text":"Наблюдение системы."}]`
		return r
	}()); err != nil {
		t.Fatalf("the linked and disputed claim must land, got %v", err)
	}

	// The summary number monotonicity UNIQUE and the case FK.
	checkDuplicate("the raced summary number", w.insertSummaryRow(t, caseID, honestRow(1)))
	_, err := adminPool.Exec(ctx, `
                insert into cases.case_summaries (tenant_id, case_id, summary_no, kind, actor_id, text, claims,
                        event_count, fact_version_count, owner_epoch, mode_version, created_at)
                values ($1,$2,1,$3,$4,$5,$6::jsonb,0,0,1,1,$7)`,
		w.tenant, nextSummaryProbeID(), casesDomain.SummaryKindAI, w.ownershipActor().ID,
		"Клиент назвал адрес доставки.", honestSummaryClaims(), ownershipNow)
	if err == nil {
		t.Fatalf("the foreign case must be refused by the FK backstop")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.SQLState() != "23503" {
		t.Fatalf("the foreign case refusal is the foreign_key_violation 23503, got %v", err)
	}

	// The watermark bounds (the server-truth columns are never negative).
	_, err = adminPool.Exec(ctx, `
                insert into cases.case_summaries (tenant_id, case_id, summary_no, kind, actor_id, text, claims,
                        event_count, fact_version_count, owner_epoch, mode_version, created_at)
                values ($1,$2,4,$3,$4,$5,$6::jsonb,-1,0,1,1,$7)`,
		w.tenant, caseID, casesDomain.SummaryKindAI, w.ownershipActor().ID,
		"Клиент назвал адрес доставки.", honestSummaryClaims(), ownershipNow)
	checkRefused("negative event count", err)

	// The command and outbox dictionary ALTERs: the summary record command
	// word and the summary event type are the legal dictionary words (the
	// dishonest forms are refused).
	if _, err := adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'record_case_summary',$3,repeat('d',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("summary-dict-probe-%d", summarySeedSeq), ownershipNow); err != nil {
		t.Fatalf("the record_case_summary command word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'record_operator_summary',$3,repeat('d',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("summary-dict-probe-%d", summarySeedSeq), ownershipNow)
	checkRefused("foreign command word", err)
	if _, err := adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.summary_recorded',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextSummaryProbeID(), caseID, nextSummaryProbeID(), nextSummaryProbeID(), ownershipNow,
		fmt.Sprintf("summary-outbox-probe-%d", summarySeedSeq), ownershipNow); err != nil {
		t.Fatalf("the case.summary_recorded event word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.summary_removed',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextSummaryProbeID(), caseID, nextSummaryProbeID(), nextSummaryProbeID(), ownershipNow,
		fmt.Sprintf("summary-outbox-probe-%d", summarySeedSeq), ownershipNow)
	checkRefused("foreign outbox event word", err)
}

// TestSummariesStoreRecordAndSnapshot covers the store-level surface of
// the summary record command and the decision context snapshot: the
// ownership gate BEFORE the material guards (a human owner gets the
// honest refusal words), the domain guard under the locks (the rune
// bounds, the byte-exact span discipline), the ref guards (every linked
// claim anchors a fact or an observation of THIS case), the SERVER
// truth watermark under the locks, the monotonic summary numbers, the
// command key conflict of the same key with another material, the
// identifiers-only outbox envelope (the summary text NEVER enters), the
// ONE-transaction snapshot read with the read-time staleness derivation
// and the perimeter absence in the fact channels.
func TestSummariesStoreRecordAndSnapshot(t *testing.T) {
	w := newCasesWorld(t, 992)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-summary-store-1", "sess-summary-store-1", `{"call":"inbound-call","from":"+79000000232"}`)
	conversationID := w.conversationFirst(t, caseID)
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}

	// One honest fact of the case (the linked claim anchor of the summary)
	// and one honest owning-system observation (the observation anchor).
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "address", Value: []byte(`"Казань, улица Кремлёвская, 1"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-2-14-address",
		IdempotencyKey: "fact-2-14-address", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, "price", "obs-2-14-price"))
	var factID, observationID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, caseID).Scan(&factID); err != nil {
		t.Fatal(err)
	}
	if err := adminPool.QueryRow(ctx, `
                select observation_id from cases.owning_system_observations
                where tenant_id=$1 and case_id=$2 and material_key='price' order by observation_no desc limit 1`,
		w.tenant, caseID).Scan(&observationID); err != nil {
		t.Fatal(err)
	}

	summaryParams := func(key string, kind, text string, claims []casesDomain.SummaryClaim) casesPort.WorkCommandParams {
		return casesPort.WorkCommandParams{
			Scope: scope, Command: "record_case_summary", Key: key,
			RequestHash: strings.Repeat("e", 64), Actor: w.ownershipActor().ID,
			CaseID: caseID, ConversationID: conversationID,
			Kind: kind, SummaryText: text, SummaryClaims: claims,
			SummaryOutboxEventID: nextSummaryProbeID(),
			CorrelationID:        nextSummaryProbeID(), Outcome: []byte(`{}`), Now: ownershipNow,
		}
	}
	summaryText := "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1. Срок доставки не назван."
	honestClaims := []casesDomain.SummaryClaim{
		{RefType: casesDomain.ClaimRefFactVersion, RefID: factID, Text: "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1."},
		{RefType: casesDomain.ClaimRefObservation, RefID: observationID, Text: "Срок доставки не назван."},
	}

	// The ownership gate BEFORE the material guards: the fresh case owns
	// the HUMAN (the ingest_initial epoch 1) — the summary is the act of
	// the owner-Staffy, the refusal carries the honest words.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, summaryParams("summary-owner-gate",
		casesDomain.SummaryKindAI, summaryText, honestClaims)), "Сводку пишет Staffy")

	// The ownership move to Staffy (the admin probe of the store gate —
	// the composed return path is the 2.13 coverage): the epoch 2 term of
	// the staffy owner over the previous human owner.
	if _, err := adminPool.Exec(ctx, `
                insert into cases.ownership_terms (tenant_id, case_id, term_id, epoch, owner_kind, owner_ref, writable_surface, origin,
                        previous_owner_kind, previous_owner_ref, actor_id, created_at)
                values ($1,$2,$3,2,'staffy',$4,'inbox','reassign','human',$4,$5,$6)`,
		w.tenant, caseID, nextSummaryProbeID(), w.ownershipActor().ID, w.ownershipActor().ID, ownershipNow); err != nil {
		t.Fatal(err)
	}

	// The domain guard under the locks: the rune bounds and the
	// byte-exact span discipline (the machine checks the structure,
	// never the truth).
	assertRuleError(t, store.ExecuteWorkCommand(ctx, summaryParams("summary-guard-runes",
		casesDomain.SummaryKindAI, strings.Repeat("а", 1001), honestClaims)), "от 1 до 1000 рун")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, summaryParams("summary-guard-span",
		casesDomain.SummaryKindAI, summaryText, []casesDomain.SummaryClaim{
			{RefType: casesDomain.ClaimRefFactVersion, RefID: factID, Text: "СРОК ДОСТАВКИ НЕ НАЗВАН."},
		})), "байт-в-байт")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, summaryParams("summary-guard-kind",
		"operator_notes", summaryText, honestClaims)), "не входит в словарь")

	// The ref guards: a foreign fact (another case) and a missing ref are
	// fabrications the store refuses (the belonging discipline).
	foreignCase, foreignWork := w.acceptedCase(t, "evt-summary-store-2", "sess-summary-store-2", `{"call":"inbound-call","from":"+79000000233"}`)
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: foreignCase, WorkItemID: foreignWork,
		MaterialKey: "address", Value: []byte(`"Москва, улица Тверская, 2"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-2-14-foreign",
		IdempotencyKey: "fact-2-14-foreign", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	var foreignFactID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, foreignCase).Scan(&foreignFactID); err != nil {
		t.Fatal(err)
	}
	assertRuleError(t, store.ExecuteWorkCommand(ctx, summaryParams("summary-ref-foreign",
		casesDomain.SummaryKindAI, summaryText, []casesDomain.SummaryClaim{
			{RefType: casesDomain.ClaimRefFactVersion, RefID: foreignFactID, Text: "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1."},
		})), "материал другого обращения")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, summaryParams("summary-ref-missing",
		casesDomain.SummaryKindAI, summaryText, []casesDomain.SummaryClaim{
			{RefType: casesDomain.ClaimRefObservation, RefID: nextSummaryProbeID(), Text: "Срок доставки не назван."},
		})), "материал другого обращения")

	// The honest record: the summary number 1, the SERVER truth watermark
	// (the request NEVER carries it) and the identifiers-only envelope.
	if err := store.ExecuteWorkCommand(ctx, summaryParams("summary-record-1",
		casesDomain.SummaryKindAI, summaryText, honestClaims)); err != nil {
		t.Fatalf("the honest summary must land, got %v", err)
	}
	// The server-truth watermark of the row: the accepted-event count, the
	// fact-version count, the owner epoch and the mode version of the
	// live case (the store took the snapshot under the locks).
	var eventCount, factVersionCount, ownerEpoch, modeVersion int64
	if err := adminPool.QueryRow(ctx, `
                select event_count, fact_version_count, owner_epoch, mode_version
                from cases.case_summaries where tenant_id=$1 and case_id=$2 and summary_no=1`,
		w.tenant, caseID).Scan(&eventCount, &factVersionCount, &ownerEpoch, &modeVersion); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 || factVersionCount != 1 || ownerEpoch != 2 || modeVersion != 1 {
		t.Fatalf("the watermark is the server truth of the moment, got events=%d facts=%d epoch=%d mode=%d",
			eventCount, factVersionCount, ownerEpoch, modeVersion)
	}

	// The identifiers-only outbox envelope: the summary text and the
	// claims NEVER enter (the perimeter of the outbox).
	var envelopeCount int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.outbox_events
                where tenant_id=$1 and event_type='case.summary_recorded'`,
		w.tenant).Scan(&envelopeCount); err != nil {
		t.Fatal(err)
	}
	var envelopePayload []byte
	if err := adminPool.QueryRow(ctx, `
                select payload from cases.outbox_events
                where tenant_id=$1 and event_type='case.summary_recorded' limit 1`,
		w.tenant).Scan(&envelopePayload); err != nil {
		t.Fatal(err)
	}
	if envelopeCount != 1 {
		t.Fatalf("exactly one summary envelope of the record, got %d", envelopeCount)
	}
	var envelope map[string]any
	if err := json.Unmarshal(envelopePayload, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tenant_id", "case_id", "summary_no", "kind", "claim_count", "linked_claims", "disputed_claims", "actor_id", "occurred_at"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("the envelope carries the identifier/counter key %s, got %s", key, envelopePayload)
		}
	}
	if strings.Contains(string(envelopePayload), "Кремлёвская") || strings.Contains(string(envelopePayload), "ref_type") {
		t.Fatalf("the summary text and the claims NEVER enter the envelope, got %s", envelopePayload)
	}

	// The command key conflict: the same key with ANOTHER material is the
	// honest conflict (the idempotency surface of 2.2..2.13).
	keyConflict := store.ExecuteWorkCommand(ctx, summaryParams("summary-record-1",
		casesDomain.SummaryKindAI, summaryText, []casesDomain.SummaryClaim{
			{Disputed: true, Text: "Срок доставки не назван."},
		}))
	if !errors.Is(keyConflict, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material is the command key conflict, got %v", keyConflict)
	}

	// The second summary: the NEXT monotonic number under the locks.
	if err := store.ExecuteWorkCommand(ctx, summaryParams("summary-record-2",
		casesDomain.SummaryKindAI, summaryText, honestClaims)); err != nil {
		t.Fatalf("the second summary must land, got %v", err)
	}

	// The ONE-transaction context snapshot: every block of the section
	// with the LAST summary and the CURRENT watermark counters (the
	// staleness derivation is the read-time domain truth — the summary
	// never mutates).
	input, err := store.ReadDecisionContext(ctx, scope, caseID, ownershipNow)
	if err != nil {
		t.Fatal(err)
	}
	if input.Summary == nil || input.Summary.SummaryNo != 2 || input.Summary.Kind != casesDomain.SummaryKindAI {
		t.Fatalf("the snapshot reads the LAST summary, got %+v", input.Summary)
	}
	if len(input.Summary.Claims) != 2 {
		t.Fatalf("the snapshot reads the claims of the last summary, got %d", len(input.Summary.Claims))
	}
	if input.Watermark.EventCount != 1 || input.Watermark.FactVersionCount != 1 ||
		input.Watermark.OwnerEpoch != 2 || input.Watermark.ModeVersion != 1 {
		t.Fatalf("the snapshot watermark is the current truth, got %+v", input.Watermark)
	}
	staleness := casesDomain.DeriveContextStaleness(input.Summary, input.Watermark)
	if staleness.Word != casesDomain.ContextStalenessActual {
		t.Fatalf("the honest derivation of the unchanged material is actual, got %+v", staleness)
	}
	// The blocks: the ownership terms (the epoch-2 staffy term), the work
	// items, the mode ladder, the facts with the confirmations, the
	// observations and the material blocks of the section.
	if len(input.OwnershipTerms) != 2 || input.CurrentOwnerEpoch != 2 {
		t.Fatalf("the snapshot reads the ownership ladder, got %d terms epoch %d", len(input.OwnershipTerms), input.CurrentOwnerEpoch)
	}
	if len(input.WorkItems) == 0 || len(input.ModeTerms) == 0 || len(input.FactVersions) != 1 || len(input.Observations) == 0 {
		t.Fatalf("the snapshot reads every block, got items=%d modes=%d facts=%d observations=%d",
			len(input.WorkItems), len(input.ModeTerms), len(input.FactVersions), len(input.Observations))
	}
	if input.HandoffRequest != nil {
		t.Fatalf("no handoff of this case — the block stays honestly absent, got %+v", input.HandoffRequest)
	}

	// The staleness probe: a NEW accepted event of the case moves the
	// watermark — the derivation reads the honest stale word with the
	// counters (the summary row itself NEVER mutates).
	w.modeTurnEvent(t, caseID, "sess-summary-store-1", "evt-summary-store-1-turn", `{"say":"когда доставка?"}`)
	after, err := store.ReadDecisionContext(ctx, scope, caseID, ownershipNow)
	if err != nil {
		t.Fatal(err)
	}
	stale := casesDomain.DeriveContextStaleness(after.Summary, after.Watermark)
	if stale.Word != casesDomain.ContextStalenessStale || stale.NewEvents != 1 {
		t.Fatalf("the new event makes the summary honestly stale with the counters, got %+v", stale)
	}
	var rowEvents int64
	if err := adminPool.QueryRow(ctx, `
                select event_count from cases.case_summaries where tenant_id=$1 and case_id=$2 and summary_no=2`,
		w.tenant, caseID).Scan(&rowEvents); err != nil {
		t.Fatal(err)
	}
	if rowEvents != 1 {
		t.Fatalf("the summary row never mutates (the watermark stays the record-time truth), got %d", rowEvents)
	}

	// The perimeter probe (the summary rows are NEVER facts): the record
	// never writes fact anchors — the live probe of the 2.11..2.13
	// precedent.
	var factRows int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.facts where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID).Scan(&factRows); err != nil {
		t.Fatal(err)
	}
	if factRows != 1 {
		t.Fatalf("the summary record never writes fact anchors (the one fact of the claim anchor stays), got %d rows", factRows)
	}
}

// matrixSequenceIDs mints the 256-id fixed sequence of the decision
// context matrix (the base 99500 lives above the summary probe space
// and far below the privacy world space — every generated id stays
// unique across the suite).
func matrixSequenceIDs() *sequenceIDs {
	values := make([]id.UUID, 0, 256)
	for i := 0; i < 256; i++ {
		values = append(values, id.MustParse(fmt.Sprintf("019a2f4c-7000-7000-8000-%012d", 99500+i)))
	}
	return &sequenceIDs{values: values}
}

// TestDecisionContextMatrix is the named integration matrix of the Story
// 2.14 operator surface: the read-time twelve-block assembly of the
// assigned operator over ONE consistent snapshot, the honest absence of
// the section without the material, the three SEPARATE fact groups with
// the provenance, the observations with the freshness words, the honest
// delivery line (acceptance ≠ delivery), the commitments with the
// receipt line, the preferences visible before the first reply, the
// recommendation projection of the last policy outcome, the honest
// NotOwner refusal of the summary at the human owner, the full
// ownership walk to the staffy owner, the summary record with the
// SERVER-truth watermark, the staleness counters of the moved material,
// the replay and the key conflict, the dictionary guards, the gates
// BEFORE the material guards, the terminal boundary of the state
// dictionary, the closed dictionaries (the medical probe) and the
// control branch of the case without the section material.
func TestDecisionContextMatrix(t *testing.T) {
	w := newCasesWorld(t, 993)
	// The matrix drives the full ownership walk (the acknowledgement
	// composes the 2.4 machinery of five identifiers, the return composes
	// another set) with TWO lifecycles, the policy machinery and the
	// producer record — the 64-id fixed pool of the world runs out
	// mid-chain. The honest rebuild precedent of the race tests: the
	// contour is rebuilt with the larger fixed sequence, every generated
	// id stays unique.
	w.cases = w.buildCasesService(matrixSequenceIDs())
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-ctx-matrix-1", "sess-ctx-matrix-1", `{"call":"inbound-call","from":"+79000000240"}`)
	conversationID := w.conversationFirst(t, caseID)
	turn := w.modeTurnEvent(t, caseID, "sess-ctx-matrix-1", "evt-ctx-matrix-1-turn", `{"say":"передайте меня специалисту"}`)

	// The context read shorthand (the review-scope operator by default).
	contextOf := func() casesAPI.DecisionContextView {
		view, err := w.cases.DecisionContext(ctx, w.ownershipActor(), w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		return view
	}
	contextOfBy := func(actor identityDomain.Principal, caseNo id.UUID) casesAPI.DecisionContextView {
		view, err := w.cases.DecisionContext(ctx, actor, w.tenant, caseNo)
		if err != nil {
			t.Fatal(err)
		}
		return view
	}
	// The producer record shorthand.
	record := func(key, text string, claims []casesDomain.SummaryClaim) (casesAPI.WorkOutcome, error) {
		return w.cases.RecordCaseSummary(ctx, casesAPI.RecordCaseSummaryCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
			Kind: casesDomain.SummaryKindAI, Text: text, Claims: claims,
			IdempotencyKey: key, Now: ownershipNow,
		})
	}

	// The material of the section: the three fact groups (the confirmed
	// address with the channel-event provenance, the unconfirmed urgency,
	// the conflicting contact_phone of two distinct values, the confirmed
	// service_kind of the policy dedup basis), one owning-system
	// observation, one draft commitment, the slowed_pace accommodation,
	// the neutral client reply (the delivery line) and the commercial
	// policy outcome of the price inquiry (the recommendation basis).
	w.recordConfirmedFact(t, caseID, workItemID, "address", `"Казань, Кремлёвская 1"`, "evt-ctx-matrix-1", "ctx-matrix-1")
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "urgency", Value: []byte(`"срочно"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "ctx-matrix-fact-urgency-src",
		Reason: "Клиент назвал срочность", IdempotencyKey: "ctx-matrix-fact-urgency", Now: casesClock.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	for i, value := range []string{`"+79000000240"`, `"+79000000241"`} {
		if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			MaterialKey: "contact_phone", Value: []byte(value),
			SourceType: casesDomain.FactSourceOperator, SourceID: fmt.Sprintf("ctx-matrix-fact-contact-src-%d", i),
			Reason:         "Два значения телефона",
			IdempotencyKey: fmt.Sprintf("ctx-matrix-fact-contact-%d", i), Now: casesClock.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	w.recordConfirmedFact(t, caseID, workItemID, "service_kind", `"диагностика"`, "evt-ctx-matrix-1", "ctx-matrix-2")
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, "price", "ctx-matrix-obs-1"))
	if _, err := w.cases.CreateCommitment(ctx, casesAPI.CreateCommitmentCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		Status: casesDomain.CommitmentStatusDraft, Terms: commitmentTerms(),
		ActionSet:  []string{casesDomain.CommitmentActionPrepareOrder},
		ValidUntil: ownershipNow.Add(24 * time.Hour), ConsumptionLimit: 1,
		IdempotencyKey: "ctx-matrix-commitment-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.RecordAccommodation(ctx, casesAPI.RecordAccommodationCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		AccommodationKey: casesDomain.AccommodationSlowedPace, AccommodationValue: "", Cleared: false,
		Note: "клиент просит не торопиться", IdempotencyKey: "ctx-matrix-accommodation-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
		Actor: w.owner, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
		ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
		Content: "Специалист подключится к обращению.", IdempotencyKey: "ctx-matrix-reply-1", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	w.policySalesPrelude(t, caseID, conversationID, turn, "ctx-matrix-policy")
	proposal := w.policySubmit(t, casesAPI.SubmitAIProposalCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
		ConversationID: conversationID, WorkItemID: workItemID, TurnRef: turn,
		Intent: casesDomain.IntentPriceInquiry, CandidateAnswer: "Диагностика от 3000",
		MissingFacts: []casesDomain.MissingFact{
			{MaterialKey: "service_kind", Reason: casesDomain.MissingReasonMandatory},
			{MaterialKey: "urgency", Reason: casesDomain.MissingReasonMandatory},
		},
		Proposer: casesDomain.ProposerSyntheticPort, IdempotencyKey: "ctx-matrix-proposal-1", Now: ownershipNow,
	})
	modeVersion, ownerEpoch, workItemVersion, factsDigest := w.policyAnchors(t, caseID, workItemID)
	w.policyEvaluateOK(t, casesAPI.EvaluatePolicyCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, ProposalID: proposal.ProposalID,
		ExpectedModeVersion: modeVersion, ExpectedOwnerEpoch: ownerEpoch,
		ExpectedFactsDigest: factsDigest, ExpectedWorkItemVersion: workItemVersion,
		IdempotencyKey: "ctx-matrix-evaluate-1", Now: ownershipNow,
	})

	// The claim anchor of the summary: the address fact of THIS case.
	var addressFactID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, caseID).Scan(&addressFactID); err != nil {
		t.Fatal(err)
	}
	summaryText := "Клиент назвал адрес доставки: Казань, Кремлёвская 1. Срочность не подтверждена источником."
	honestClaims := []casesDomain.SummaryClaim{
		{RefType: casesDomain.ClaimRefFactVersion, RefID: addressFactID, Text: "Клиент назвал адрес доставки: Казань, Кремлёвская 1."},
		{Disputed: true, Text: "Срочность не подтверждена источником."},
	}

	// Row 1 — the honest absence: the material blocks exist (the facts, the
	// observations, the commitments, the preferences), but NO open handoff
	// and NO summary — the section stays absent (Present=false), the page
	// renders the unchanged case (the byte-for-byte control branch of the
	// render smoke).
	t.Run("row-01 honest absence: no handoff and no summary = no section", func(t *testing.T) {
		view := contextOf()
		if view.Present || view.Handoff.Present || view.Summary.Present {
			t.Fatalf("the section appears only with the open handoff or the summary, got %+v", view)
		}
		if len(view.Facts.Confirmed) == 0 || len(view.Facts.Unconfirmed) == 0 || len(view.Facts.Conflicting) == 0 {
			t.Fatalf("the material blocks read honestly without the section flag, got %+v", view.Facts)
		}
	})

	// Row 2 — the three SEPARATE fact groups with the provenance (AC-1):
	// confirmed (address, service_kind), unconfirmed (urgency) and
	// conflicting (contact_phone — the read-time derivation, never a
	// stored status); every row carries the status text, the version and
	// the provenance (the source type, the source event, the channel).
	t.Run("row-02 facts: three separate groups with provenance and version", func(t *testing.T) {
		view := contextOf()
		statusOf := func(rows []casesAPI.ContextFactRow, key string) (casesAPI.ContextFactRow, bool) {
			for _, row := range rows {
				if row.MaterialKey == key {
					return row, true
				}
			}
			return casesAPI.ContextFactRow{}, false
		}
		address, ok := statusOf(view.Facts.Confirmed, "address")
		if !ok || address.Status != casesDomain.ContextFactConfirmed || address.VersionNo != 1 ||
			address.StatusText != casesDomain.ContextFactStatusText(casesDomain.ContextFactConfirmed) ||
			address.SourceType != casesDomain.FactSourceChannelEvent || address.SourceID == "" || address.Channel != "phone" {
			t.Fatalf("the confirmed address row carries the provenance and the version, got %+v", address)
		}
		if _, ok := statusOf(view.Facts.Confirmed, "service_kind"); !ok {
			t.Fatalf("the confirmed group carries the dedup basis fact, got %+v", view.Facts.Confirmed)
		}
		urgency, ok := statusOf(view.Facts.Unconfirmed, "urgency")
		if !ok || urgency.Status != casesDomain.ContextFactUnconfirmed || urgency.SourceType != casesDomain.FactSourceOperator {
			t.Fatalf("the unconfirmed urgency row renders the honest mirror word, got %+v", urgency)
		}
		phone, ok := statusOf(view.Facts.Conflicting, "contact_phone")
		if !ok || phone.Status != casesDomain.ContextFactConflicting || phone.VersionNo != 2 {
			t.Fatalf("the conflicting contact_phone derives read-time with the latest version, got %+v", phone)
		}
		// The conflicting status is NEVER stored: the probe reads the raw
		// column absence of any status word of the fact material.
		var statusRows int
		if err := adminPool.QueryRow(ctx, `
                        select count(*) from information_schema.columns
                        where table_schema='cases' and table_name='facts' and column_name='status'`).Scan(&statusRows); err != nil {
			t.Fatal(err)
		}
		if statusRows != 0 {
			t.Fatalf("the fact status word is a read-time derivation — the stored status column must not exist, got %d", statusRows)
		}
	})

	// Row 3 — the observations block (AC-2): the Owning System word, the
	// provider-observed time, the freshness word of the matrix window, the
	// conflict rule and the evidence status (the provider technique stays
	// behind the progressive disclosure of the page).
	t.Run("row-03 observations: owning system, freshness, conflict rule, evidence", func(t *testing.T) {
		view := contextOf()
		if len(view.Observations) != 1 {
			t.Fatalf("the observation block reads the one read row, got %+v", view.Observations)
		}
		observation := view.Observations[0]
		if observation.MaterialKey != "price" || observation.OwningSystem == "" ||
			observation.FreshnessWord == "" || observation.ConflictRule == "" || observation.EvidenceStatus == "" {
			t.Fatalf("the observation row carries the honest words, got %+v", observation)
		}
	})

	// Row 4 — the delivery lines (AC-2): the dispatch truth is the 2.4
	// drain OUTCOME word — exactly the truth the 2.12 panel reads (the
	// LEFT JOIN of the ownership surface). The acceptance by the machine
	// is NOT the delivery: the undrained reply stays prepared (nothing
	// has left); the drained reply without the evidence event is the
	// honest unknown (the record of a record is not a receipt).
	t.Run("row-04 deliveries: acceptance is not delivery", func(t *testing.T) {
		view := contextOf()
		if len(view.Deliveries) != 1 {
			t.Fatalf("the delivery block reads the one reply, got %+v", view.Deliveries)
		}
		if view.Deliveries[0].State != casesDomain.ReplyDeliveryPrepared || view.Deliveries[0].LastKind != "" {
			t.Fatalf("the undrained reply is the honest prepared (nothing has left), got %+v", view.Deliveries[0])
		}
		// The drain writes the dispatched OUTCOME — the record of the
		// record: the reply becomes the honest unknown, never delivered.
		drained, err := w.cases.DrainClientReplies(ctx, w.ownershipActor(), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if drained.Considered != 1 || drained.Dispatched != 1 || drained.Stale != 0 {
			t.Fatalf("the one fresh reply drains dispatched, got %+v", drained)
		}
		view = contextOf()
		if view.Deliveries[0].State != casesDomain.ReplyDeliveryUnknown || view.Deliveries[0].LastKind != "" {
			t.Fatalf("the dispatched reply without the evidence is the honest unknown, got %+v", view.Deliveries[0])
		}
		// The stale probe material: this second reply stays undrained — the
		// ownership walk of row-09 moves the case under it, the later drain
		// marks it stale_needs_review (it never left either — row-09 pins
		// the honest prepared again).
		if _, err := w.cases.SendClientReply(ctx, casesAPI.SendClientReplyCommand{
			Actor: w.owner, TenantID: w.tenant, CaseID: caseID, ConversationID: conversationID,
			ExpectedEpoch: 1, ReplyClass: casesDomain.ReplyClassNeutral,
			Content: "Специалист подключится к обращению повторно.", IdempotencyKey: "ctx-matrix-reply-2", Now: ownershipNow,
		}); err != nil {
			t.Fatal(err)
		}
	})

	// Row 5 — the commitments block (AC-3): the current version with the
	// status word, the validity time and the honest absent receipt line
	// (the Action Gateway is Epic 3 — the line is the truth, never a
	// guess).
	t.Run("row-05 commitments: version, status, valid until, receipt absent", func(t *testing.T) {
		view := contextOf()
		if len(view.Commitments) != 1 {
			t.Fatalf("the commitment block reads the one commitment, got %+v", view.Commitments)
		}
		commitment := view.Commitments[0]
		if commitment.VersionNo != 1 || commitment.Status != casesDomain.ContextCommitmentDraft ||
			commitment.ValidUntil.IsZero() || commitment.ReceiptText != casesDomain.ReceiptAbsentText {
			t.Fatalf("the draft commitment carries the honest words with the absent receipt, got %+v", commitment)
		}
		if len(commitment.TermsKnown) == 0 {
			t.Fatalf("the commitment row carries the known mandatory terms, got %+v", commitment)
		}
	})

	// Row 6 — the preferences block (AC-4): the slowed_pace accommodation
	// is visible BEFORE the first reply of the assignee (the reply of this
	// row's world is the OWNER's status message — the assignee has not
	// answered yet).
	t.Run("row-06 preferences: slowed_pace visible before the assignee reply", func(t *testing.T) {
		view := contextOf()
		if len(view.Preferences.AccommodationKeys) != 1 ||
			view.Preferences.AccommodationKeys[0] != casesDomain.AccommodationSlowedPace {
			t.Fatalf("the current accommodation set carries the slowed_pace key, got %+v", view.Preferences)
		}
	})

	// Row 7 — the ask and the assignment: the open handoff of the assigned
	// round with the human owner (назначение ≠ принятие — the summary is
	// NOT yet writable); the AC-1 blocks of the assigned operator: the
	// reason word with the anchor turn and the deadline, the active work
	// items, the mode word with the version, the assurance words and the
	// owner word with the epoch.
	t.Run("row-07 ask+assign: the AC-1 blocks to the assigned operator BEFORE ack", func(t *testing.T) {
		if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, TurnRef: turn,
			ExpectedModeVersion: 2, TargetRoles: []string{casesDomain.HandoffTargetRoleOperator},
			AckDeadline: casesDomain.HandoffAck1h, IdempotencyKey: "ctx-matrix-ask-1", Now: ownershipNow,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.cases.AssignHumanHandoff(ctx, casesAPI.AssignHumanHandoffCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID,
			Assignee: w.backup, Backup: w.owner.ID, AckDeadline: casesDomain.HandoffAck1h,
			IdempotencyKey: "ctx-matrix-assign-1", Now: ownershipNow,
		}); err != nil {
			t.Fatal(err)
		}
		view := contextOfBy(w.backupActor(), caseID)
		if !view.Present || !view.Handoff.Present {
			t.Fatalf("the open handoff makes the section, got %+v", view)
		}
		handoff := view.Handoff
		if handoff.Reason != casesDomain.HandoffReasonClientRequest || handoff.State != casesDomain.HandoffStateAssigned ||
			handoff.TurnRef != turn.String() || handoff.AckDeadline != casesDomain.HandoffAck1h || handoff.DeadlineAt.IsZero() {
			t.Fatalf("the handoff block carries the reason, the state, the anchor and the deadline, got %+v", handoff)
		}
		if handoff.ReasonText != casesDomain.HandoffReasonText(casesDomain.HandoffReasonClientRequest) ||
			handoff.StateText != casesDomain.HandoffStateText(casesDomain.HandoffStateAssigned) {
			t.Fatalf("the handoff block carries the honest texts, got %+v", handoff)
		}
		if !view.Mode.Present || view.Mode.Mode != casesDomain.ModeHumanRequested || view.Mode.ModeVersion != 3 {
			t.Fatalf("the mode block carries the ratcheted word with the version, got %+v", view.Mode)
		}
		if !view.Owner.Present || view.Owner.OwnerKind != casesDomain.OwnerKindHuman || view.Owner.Epoch != 1 {
			t.Fatalf("the owner stays the human of the epoch 1 (назначение ≠ принятие), got %+v", view.Owner)
		}
		if view.Assurance.IdentityLevel == "" || view.Assurance.AuthorityLevel == "" {
			t.Fatalf("the assurance words render honestly (the none of the absence), got %+v", view.Assurance)
		}
		if len(view.WorkItems) == 0 || view.WorkItems[0].State != casesDomain.WorkItemStateUntriaged {
			t.Fatalf("the active work items render the non-terminal words, got %+v", view.WorkItems)
		}
		if view.Summary.Present {
			t.Fatalf("the summary is honestly absent before the record, got %+v", view.Summary)
		}
	})

	// Row 8 — the honest NotOwner refusal of the summary at the human
	// owner (the summary is the act of the owner-Staffy of the moment —
	// before the acknowledgement AND after it the human owns the case).
	t.Run("row-08 record at the human owner: the honest NotOwner words", func(t *testing.T) {
		_, err := record("ctx-matrix-summary-owner-1", summaryText, honestClaims)
		assertRuleError(t, err, "Сводку пишет Staffy")
	})

	// Row 9 — the ownership walk: the assignee acknowledges (the epoch 2
	// human owner), then returns the case to Staffy (the epoch 3 staffy
	// owner — the composed 2.4 machinery); the second unconditional ask
	// opens the lifecycle again under the staffy owner.
	t.Run("row-09 ack + return + the second ask: the staffy owner with the open handoff", func(t *testing.T) {
		if _, err := w.cases.AcknowledgeHumanHandoff(ctx, casesAPI.AcknowledgeHumanHandoffCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID,
			ExpectedEpoch: 1, IdempotencyKey: "ctx-matrix-ack-1", Now: ownershipNow,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.cases.ReturnToStaffy(ctx, casesAPI.ReturnToStaffyCommand{
			Actor: w.backupActor(), TenantID: w.tenant, CaseID: caseID,
			Result: casesDomain.HandoffResultIssueResolved, ExpectedEpoch: 2,
			IdempotencyKey: "ctx-matrix-return-1", Now: ownershipNow,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
			Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, TurnRef: turn,
			ExpectedModeVersion: 3, TargetRoles: []string{casesDomain.HandoffTargetRoleOperator},
			AckDeadline: casesDomain.HandoffAck1h, IdempotencyKey: "ctx-matrix-ask-2", Now: ownershipNow,
		}); err != nil {
			t.Fatal(err)
		}
		view := contextOf()
		if !view.Owner.Present || view.Owner.OwnerKind != casesDomain.OwnerKindStaffy || view.Owner.Epoch != 3 {
			t.Fatalf("the staffy owner of the epoch 3 with the open handoff, got %+v", view.Owner)
		}
		if !view.Handoff.Present || view.Handoff.State != casesDomain.HandoffStateRequested {
			t.Fatalf("the second ask opens the lifecycle under the staffy owner, got %+v", view.Handoff)
		}
		// The stale reply of the row-04 material never left either: the
		// reply was recorded under the epoch-1 staffy term — the walk moved
		// the case (the epochs 2 and 3) — the drain marks it
		// stale_needs_review (the payload is never relayed); the delivery
		// state stays the honest prepared: without the dispatched outcome
		// nothing has left (the 2.4 truth the 2.12 panel of the same page
		// reads — the two surfaces never contradict each other).
		staleDrained, err := w.cases.DrainClientReplies(ctx, w.ownershipActor(), w.tenant)
		if err != nil {
			t.Fatal(err)
		}
		if staleDrained.Considered != 1 || staleDrained.Dispatched != 0 || staleDrained.Stale != 1 {
			t.Fatalf("the reply of the moved ownership drains stale_needs_review, got %+v", staleDrained)
		}
		view = contextOf()
		if len(view.Deliveries) != 2 {
			t.Fatalf("the delivery block reads the two replies, got %+v", view.Deliveries)
		}
		if view.Deliveries[1].State != casesDomain.ReplyDeliveryPrepared || view.Deliveries[1].LastKind != "" {
			t.Fatalf("the stale_needs_review reply never left — the honest prepared, got %+v", view.Deliveries[1])
		}
	})

	// Row 10 — the honest record: the summary number 1, the kind word, the
	// SERVER-truth watermark (the request never carries the counters — the
	// store snapshot under the locks) and the identifiers-only outbox
	// envelope.
	t.Run("row-10 record: summary_no=1, the server watermark, the outbox envelope", func(t *testing.T) {
		var events, factVersions, modeTerms int64
		if err := adminPool.QueryRow(ctx, `
                        select (select count(*) from cases.channel_events where tenant_id=$1 and case_id=$2 and state='accepted'),
                               (select count(*) from cases.fact_versions where tenant_id=$1 and case_id=$2),
                               (select coalesce(max(mode_version),0) from cases.mode_terms where tenant_id=$1 and case_id=$2)`,
			w.tenant, caseID).Scan(&events, &factVersions, &modeTerms); err != nil {
			t.Fatal(err)
		}
		outcome, err := record("ctx-matrix-summary-1", summaryText, honestClaims)
		if err != nil || outcome.Command != "record_case_summary" || outcome.SummaryKind != casesDomain.SummaryKindAI ||
			outcome.SummaryClaims != 2 || outcome.Replay {
			t.Fatalf("the honest record answers the command material, got %+v %v", outcome, err)
		}
		var rowKind string
		var watermark [4]int64
		if err := adminPool.QueryRow(ctx, `
                        select kind, event_count, fact_version_count, owner_epoch, mode_version
                        from cases.case_summaries where tenant_id=$1 and case_id=$2 and summary_no=1`,
			w.tenant, caseID).Scan(&rowKind, &watermark[0], &watermark[1], &watermark[2], &watermark[3]); err != nil {
			t.Fatal(err)
		}
		if rowKind != casesDomain.SummaryKindAI || watermark[0] != events || watermark[1] != factVersions ||
			watermark[2] != 3 || watermark[3] != modeTerms {
			t.Fatalf("the watermark is the server truth under the locks (events=%d facts=%d epoch=3 mode=%d), got %v",
				events, factVersions, modeTerms, watermark)
		}
		var envelope string
		if err := adminPool.QueryRow(ctx, `
                        select payload from cases.outbox_events
                        where tenant_id=$1 and event_type='case.summary_recorded'`, w.tenant).Scan(&envelope); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(envelope, "Кремлёвская") || strings.Contains(envelope, "ref_type") {
			t.Fatalf("the summary text and the claims NEVER enter the envelope, got %s", envelope)
		}
	})

	// Row 11 — the summary view (AC-5/AC-6): the marking text of the
	// AI-derived representation, the claims with the byte-exact spans and
	// the refs, and the honest actual word of the unchanged material.
	t.Run("row-11 summary view: marking, claims, actual", func(t *testing.T) {
		view := contextOf()
		summary := view.Summary
		if !summary.Present || summary.SummaryNo != 1 || summary.Kind != casesDomain.SummaryKindAI ||
			summary.Text != summaryText || summary.MarkingText != casesDomain.AIMarkingText {
			t.Fatalf("the summary block carries the material with the honest marking, got %+v", summary)
		}
		if len(summary.Claims) != 2 || summary.Claims[0].RefID != addressFactID.String() ||
			!summary.Claims[1].Disputed {
			t.Fatalf("the claims carry the ref and the disputed mark, got %+v", summary.Claims)
		}
		if summary.Staleness != casesDomain.ContextStalenessActual || summary.StalenessCountersText != "" {
			t.Fatalf("the unchanged material derives the honest actual word, got %+v", summary)
		}
	})

	// Row 12 — the recommendation projection (AC-7): the plan step words
	// of the LAST policy outcome with the reason, the mandatory checks,
	// the deadline of the open handoff round and the constraint words of
	// the effective mode — the projection NEVER grants an authority (the
	// honest boundary text renders beside it).
	t.Run("row-12 recommendation: the projection of the last outcome", func(t *testing.T) {
		view := contextOf()
		if !view.Recommendation.Present {
			t.Fatalf("the last policy outcome projects the recommendation, got %+v", view.Recommendation)
		}
		section := policySectionOf(t, w.policyView(t, caseID), workItemID)
		if section.Current == nil {
			t.Fatal("the policy view carries the current outcome of the work item")
		}
		if len(view.Recommendation.Steps) != len(section.Current.NextSteps) {
			t.Fatalf("the steps are the plan words of the last outcome, got %+v want %+v",
				view.Recommendation.Steps, section.Current.NextSteps)
		}
		if view.Recommendation.Reason != section.Current.Reason {
			t.Fatalf("the reason is the plan reason word, got %q want %q", view.Recommendation.Reason, section.Current.Reason)
		}
		if len(view.Recommendation.Checks) != len(section.Current.MandatoryFacts) {
			t.Fatalf("the checks are the mandatory facts of the last outcome, got %+v", view.Recommendation.Checks)
		}
		if view.Recommendation.AckDeadline != casesDomain.HandoffAck1h || view.Recommendation.DeadlineAt.IsZero() {
			t.Fatalf("the deadline is the open handoff round, got %+v", view.Recommendation)
		}
		if view.Recommendation.Mode != casesDomain.ModeHumanRequested {
			t.Fatalf("the constraint words are the effective mode, got %+v", view.Recommendation)
		}
		if view.Recommendation.ProjectionText != casesDomain.RecommendationProjectionText {
			t.Fatalf("the honest projection boundary text renders, got %+v", view.Recommendation)
		}
	})

	// Row 13 — the projection creates NO records (AC-7): the read of the
	// context writes nothing — the recommendation is a projection, the
	// deviation is any permitted command with its own record.
	t.Run("row-13 the projection writes nothing", func(t *testing.T) {
		var before, after int64
		if err := adminPool.QueryRow(ctx, `
                        select count(*) from cases.channel_events where tenant_id=$1 and case_id=$2 and state='accepted'`,
			w.tenant, caseID).Scan(&before); err != nil {
			t.Fatal(err)
		}
		contextOf()
		if err := adminPool.QueryRow(ctx, `
                        select count(*) from cases.channel_events where tenant_id=$1 and case_id=$2 and state='accepted'`,
			w.tenant, caseID).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatalf("the context read writes no records, got %d then %d", before, after)
		}
	})

	// Row 14 — the staleness of the moved material (AC-6): a NEW accepted
	// client event moves the watermark — the honest stale word with the
	// counters («новых событий: 1»), the summary row itself NEVER mutates.
	t.Run("row-14 stale by the new event: the honest counters, no mutation", func(t *testing.T) {
		w.modeTurnEvent(t, caseID, "sess-ctx-matrix-1", "evt-ctx-matrix-1-turn-2", `{"say":"когда доставка?"}`)
		view := contextOf()
		if view.Summary.Staleness != casesDomain.ContextStalenessStale {
			t.Fatalf("the new event derives the honest stale word, got %+v", view.Summary)
		}
		if !strings.Contains(view.Summary.StalenessCountersText, "новых событий: 1") {
			t.Fatalf("the counters tell what moved, got %q", view.Summary.StalenessCountersText)
		}
		var rowEvents int64
		if err := adminPool.QueryRow(ctx, `
                        select event_count from cases.case_summaries where tenant_id=$1 and case_id=$2 and summary_no=1`,
			w.tenant, caseID).Scan(&rowEvents); err != nil {
			t.Fatal(err)
		}
		if rowEvents == 0 {
			t.Fatalf("the summary row keeps the record-time watermark, got %d", rowEvents)
		}
	})

	// Row 15 — the staleness of the new fact version: the counters
	// accumulate the honest dimensions (the events AND the fact versions).
	t.Run("row-15 stale by the new fact version: the counters accumulate", func(t *testing.T) {
		if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			MaterialKey: "urgency", Value: []byte(`"не срочно"`),
			SourceType: casesDomain.FactSourceOperator, SourceID: "ctx-matrix-fact-urgency-src-2",
			Reason:         "Клиент уточнил срочность",
			IdempotencyKey: "ctx-matrix-fact-urgency-2", Now: casesClock.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		view := contextOf()
		if view.Summary.Staleness != casesDomain.ContextStalenessStale ||
			!strings.Contains(view.Summary.StalenessCountersText, "новых версий фактов: 1") {
			t.Fatalf("the new fact version accumulates the honest counter, got %q", view.Summary.StalenessCountersText)
		}
	})

	// Row 16 — the replay and the key conflict: the exact repeat of the
	// same material answers the stored outcome; the same key with another
	// material is the honest 409.
	t.Run("row-16 replay + key conflict", func(t *testing.T) {
		replay, err := record("ctx-matrix-summary-1", summaryText, honestClaims)
		if err != nil || !replay.Replay || replay.Command != "record_case_summary" {
			t.Fatalf("the exact repeat replays the stored outcome, got %+v %v", replay, err)
		}
		_, err = record("ctx-matrix-summary-1", summaryText, []casesDomain.SummaryClaim{
			{Disputed: true, Text: "Срочность не подтверждена источником."},
		})
		if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
			t.Fatalf("the same key with another material is the key conflict, got %v", err)
		}
	})

	// Row 17 — the app guards of the producer material: the paraphrase of
	// the claim text (the byte-exact span discipline) and the unlinked
	// undisputed claim refuse with the honest 400 rule words.
	t.Run("row-17 guards: the byte-exact span and the linked-or-disputed discipline", func(t *testing.T) {
		_, err := record("ctx-matrix-guard-span-1", summaryText, []casesDomain.SummaryClaim{
			{RefType: casesDomain.ClaimRefFactVersion, RefID: addressFactID, Text: "СРОЧНОСТЬ НЕ ПОДТВЕРЖДЕНА ИСТОЧНИКОМ."},
		})
		assertRuleError(t, err, "байт-в-байт")
		_, err = record("ctx-matrix-guard-link-1", summaryText, []casesDomain.SummaryClaim{
			{Text: "Срочность не подтверждена источником."},
		})
		assertRuleError(t, err, "не связано и не помечено спорным")
	})

	// Row 18 — the gates BEFORE the material guards (the M1 precedent):
	// the audit-only member is forbidden on the read AND the record (the
	// dictionary violation never masks the scope refusal); the member of
	// another tenant reads the plain 404; the non-member the plain 404.
	t.Run("row-18 gates: audit-only 403 BEFORE guards; foreign tenant and non-member 404", func(t *testing.T) {
		auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "ctx-matrix-audit@example.org", "expert_qa",
			[]identityDomain.Scope{identityDomain.Audit}, "000000009931", "000000009932")
		auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
		if _, err := w.cases.DecisionContext(ctx, auditOnly, w.tenant, caseID); !errors.Is(err, identityDomain.ErrForbidden) ||
			!strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("the audit-only member is forbidden on the read, got %v", err)
		}
		if _, err := w.cases.RecordCaseSummary(ctx, casesAPI.RecordCaseSummaryCommand{
			Actor: auditOnly, TenantID: w.tenant, CaseID: caseID,
			Kind: "operator_notes", Text: summaryText, Claims: honestClaims,
			IdempotencyKey: "ctx-matrix-audit-1", Now: ownershipNow,
		}); !errors.Is(err, identityDomain.ErrForbidden) {
			t.Fatalf("the scope refusal never yields to the dictionary violation, got %v", err)
		}
		foreignTenant := createForeignTenant(t, w)
		foreign := inviteMember(t, w.identity, w.owner, foreignTenant, "ctx-matrix-foreign@example.org", "expert_qa",
			[]identityDomain.Scope{identityDomain.Review}, "000000009933", "000000009934")
		if _, err := w.cases.DecisionContext(ctx, foreign, w.tenant, caseID); !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the member of another tenant reads the plain 404, got %v", err)
		}
		if _, err := w.cases.RecordCaseSummary(ctx, casesAPI.RecordCaseSummaryCommand{
			Actor: foreign, TenantID: w.tenant, CaseID: caseID,
			Kind: casesDomain.SummaryKindAI, Text: summaryText, Claims: honestClaims,
			IdempotencyKey: "ctx-matrix-foreign-1", Now: ownershipNow,
		}); !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the foreign member never records, got %v", err)
		}
		nonMember := actorPrincipal{principalID: nextSummaryProbeID(), stepUp: true}.principal()
		if _, err := w.cases.DecisionContext(ctx, nonMember, w.tenant, caseID); !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
			t.Fatalf("the non-member reads the plain 404, got %v", err)
		}
	})

	// Row 19 — the terminal boundary of the state dictionary: the slice
	// carries the open word only — the terminal case is structurally
	// impossible (the honest CHECK refusal; the record guard stays for the
	// future dictionary growth — the 2.11/2.12/2.13 precedent).
	t.Run("row-19 terminal boundary: the state dictionary keeps the boundary honest", func(t *testing.T) {
		_, err := adminPool.Exec(ctx, `
                        update cases.cases set state='resolved' where tenant_id=$1 and case_id=$2`,
			w.tenant, caseID)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("the terminal state word is impossible in this slice (the CHECK refusal), got %v", err)
		}
	})

	// Row 20 — the closed dictionaries (the medical probe of AC-4): the
	// kind, the ref types, the staleness words and the fact statuses are
	// the exact fixed lists — no runtime breeding, no medical words.
	t.Run("row-20 dictionaries closed: the exact words, no medical vocabulary", func(t *testing.T) {
		if len(casesDomain.SummaryKinds) != 1 || casesDomain.SummaryKinds[0] != casesDomain.SummaryKindAI {
			t.Fatalf("the kind dictionary carries the single producer word, got %v", casesDomain.SummaryKinds)
		}
		if len(casesDomain.ClaimRefTypes) != 2 {
			t.Fatalf("the ref-type dictionary carries the two material words, got %v", casesDomain.ClaimRefTypes)
		}
		if len(casesDomain.ContextStalenessWords) != 2 {
			t.Fatalf("the staleness dictionary carries the two honest words, got %v", casesDomain.ContextStalenessWords)
		}
		if len(casesDomain.ContextFactStatuses) != 3 {
			t.Fatalf("the fact-status dictionary carries the three render words, got %v", casesDomain.ContextFactStatuses)
		}
		for _, word := range casesDomain.AccommodationKeys {
			if strings.Contains(word, "medical") || strings.Contains(word, "diagnosis") || strings.Contains(word, "лечен") {
				t.Fatalf("the accommodation dictionary carries no medical words, got %q", word)
			}
		}
	})

	// Row 21 — the control branch: a case without the open handoff and
	// without the summary renders NO section (the honest absence — the
	// page stays byte-for-byte unchanged, the render smoke pins the
	// boundary).
	t.Run("row-21 control branch: no material = no section", func(t *testing.T) {
		caseB, _ := w.acceptedCase(t, "evt-ctx-matrix-2", "sess-ctx-matrix-2", `{"call":"inbound-call","from":"+79000000241"}`)
		view := contextOfBy(w.ownershipActor(), caseB)
		if view.Present || view.Handoff.Present || view.Summary.Present {
			t.Fatalf("the fresh case carries no section material, got %+v", view)
		}
	})
}
