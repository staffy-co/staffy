//go:build integration

package integration_test

import (
        "context"
        "encoding/json"
        "errors"
        "fmt"
        "strings"
        "testing"

        casesAPI "github.com/artemiosu/staffy/internal/cases/api"
        casesDomain "github.com/artemiosu/staffy/internal/cases/domain"
        casesPort "github.com/artemiosu/staffy/internal/cases/port"
        casesStore "github.com/artemiosu/staffy/internal/cases/store"
        "github.com/artemiosu/staffy/internal/platform/id"
        "github.com/artemiosu/staffy/internal/platform/tenancy"
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
        summaryNo   int64
        kind        string
        text        string
        claims      string
        negativeUse bool
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
