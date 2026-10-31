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

	"github.com/jackc/pgx/v5/pgconn"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesPort "github.com/staffy-co/staffy/internal/cases/port"
	casesStore "github.com/staffy-co/staffy/internal/cases/store"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/staffy-co/staffy/internal/platform/tenancy"
)

// The Story 2.15 step-02 probes: the migration 00015 invariants are
// pinned by the live probes immediately (the review R1 lesson of 2.7 —
// the invariant is never only a domain guard). Three probe families:
// the pg_class/RLS/grants discipline of the append-only correction
// table, the INSERT backstops (the outcome dictionary, the span array
// shape, the reason bounds, the version pin, the summary link, the
// times discipline, the number/id uniqueness, the fact-version and
// summary FKs, the command/outbox dictionary ALTERs), and the
// store-level probes (the target guards, the domain guard under the
// locks with the byte-exact spans against the cited source, the matrix
// gate — the disputed outcome writes NO fact version, the applied pour
// through the existing record discipline with the correction source
// key, the identifiers-only outbox envelope, the command key conflict,
// the monotonic correction numbers and the ONE-transaction snapshot of
// the context material together with the correction rows).

// correctionSeedSeq builds the distinct probe identifiers of the
// correction surface (the 99400 range lives above the summary probe
// space 99300.. and below the context matrix pool 99500.. — the ranges
// are unique per the 2.14 lesson).
var correctionSeedSeq int

func nextCorrectionProbeID() id.UUID {
	correctionSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-7000-7000-8000-%012d", 99400+correctionSeedSeq))
}

// TestCorrectionsTablesGrantsAndRLS asserts the append-only discipline
// of the correction surface at the database level: RLS + FORCE,
// INSERT/SELECT for staffy_app and no UPDATE, DELETE or TRUNCATE (the
// degradation, handoff and summary precedents).
func TestCorrectionsTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('case_corrections')`)
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
	if state, ok := seen["case_corrections"]; !ok || !state[0] || !state[1] {
		t.Fatalf("case_corrections must carry RLS + FORCE (the append-only discipline), got %+v", state)
	}
	for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
		var allowed bool
		if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases.case_corrections", grant).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if (grant == "INSERT" || grant == "SELECT") != allowed {
			t.Fatalf("%s on cases.case_corrections: allowed=%v, want %v", grant, allowed, grant == "INSERT" || grant == "SELECT")
		}
	}
}

// insertCorrectionRow runs the direct INSERT of one correction row over
// the live fact anchor (the admin role behind the RLS curtain — the
// DB-probe precedent of 2.7..2.14).
func (w *casesWorld) insertCorrectionRow(t *testing.T, caseID, factID id.UUID, correctionID id.UUID, no int64, outcome, spans string, reason string, summaryNo any) error {
	t.Helper()
	_, err := adminPool.Exec(context.Background(), `
                insert into cases.case_corrections (tenant_id, case_id, correction_id, correction_no, fact_id, version_no,
                        summary_no, proposed_value, spans, outcome, actor_id, channel, reason, observed_at, received_at, created_at)
                values ($1,$2,$3,$4,$5,1,$6,'"Казань"'::jsonb,$7::jsonb,$8,$9,'operator',$10,$11,$11,$11)`,
		w.tenant, caseID, correctionID, no, factID, summaryNo, spans, outcome,
		w.ownershipActor().ID, reason, ownershipNow)
	return err
}

// TestCorrectionsDBConstraintsRefuseDishonestRows pins the database
// backstops of the 00015 surface: the outcome dictionary (applied or
// disputed — the refusal cases are the 400 guards, never words), the
// span array shape (0..8 elements, every one a non-empty string), the
// reason bounds (1..200), the version pin (>= 1), the summary link
// (NULL or >= 1, the FK to the summary of THIS case), the times
// discipline (received >= observed - 48h), the correction number and id
// uniqueness, the fact-version FK and the command/outbox dictionary
// ALTERs — the honest shape is a database guarantee, not only a domain
// guard.
func TestCorrectionsDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 994)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-correction-db-1", "sess-correction-db-1", `{"call":"inbound-call","from":"+79000000401"}`)
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "address", Value: []byte(`"Казань, улица Кремлёвская, 1"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-2-15-db-address",
		IdempotencyKey: "fact-2-15-db-address", Now: ownershipNow,
	}); err != nil {
		t.Fatal(err)
	}
	var factID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, caseID).Scan(&factID); err != nil {
		t.Fatal(err)
	}

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
	// checkForeign pins the foreign_key_violation refusal.
	checkForeign := func(probe string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the foreign row must be refused, got accepted", probe)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23503" {
			t.Fatalf("%s: expected the foreign_key_violation 23503, got %v", probe, err)
		}
	}

	// The outcome dictionary: the refusal cases are the 400 guards, never
	// outcome words — the table admits applied and disputed ONLY.
	checkRefused("foreign outcome word", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 1, "rejected", `[]`, "оператор сверил источник", nil))
	checkRefused("automatic confirmation outcome", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 1, "confirmed", `[]`, "оператор сверил источник", nil))
	appliedRowID := nextCorrectionProbeID()
	if err := w.insertCorrectionRow(t, caseID, factID, appliedRowID, 1, casesDomain.CorrectionOutcomeApplied, `[]`, "оператор сверил источник", nil); err != nil {
		t.Fatalf("the honest applied row must land, got %v", err)
	}
	if err := w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 2, casesDomain.CorrectionOutcomeDisputed, `["доставка нужна к пятнице"]`, "оператор сверил источник", nil); err != nil {
		t.Fatalf("the honest disputed row with a span must land, got %v", err)
	}

	// The span array shape: 0..8 elements, every one a NON-EMPTY string.
	checkRefused("nine spans", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 3, casesDomain.CorrectionOutcomeApplied,
		`["а","а","а","а","а","а","а","а","а"]`, "оператор сверил источник", nil))
	checkRefused("non-string span", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 3, casesDomain.CorrectionOutcomeApplied,
		`[42]`, "оператор сверил источник", nil))
	checkRefused("empty span", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 3, casesDomain.CorrectionOutcomeApplied,
		`[""]`, "оператор сверил источник", nil))
	checkRefused("spans not an array", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 3, casesDomain.CorrectionOutcomeApplied,
		`"доставка"`, "оператор сверил источник", nil))

	// The reason bounds: the act is conscious, the why is mandatory.
	checkRefused("empty reason", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 3, casesDomain.CorrectionOutcomeApplied,
		`[]`, "", nil))
	checkRefused("201-rune reason", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 3, casesDomain.CorrectionOutcomeApplied,
		`[]`, strings.Repeat("а", 201), nil))

	// The version pin and the summary link shape.
	_, err := adminPool.Exec(ctx, `
                insert into cases.case_corrections (tenant_id, case_id, correction_id, correction_no, fact_id, version_no,
                        summary_no, proposed_value, spans, outcome, actor_id, channel, reason, observed_at, received_at, created_at)
                values ($1,$2,$3,3,$4,0,null,'"Казань"'::jsonb,'[]'::jsonb,$5,$6,'operator','оператор сверил источник',$7,$7,$7)`,
		w.tenant, caseID, nextCorrectionProbeID(), factID, casesDomain.CorrectionOutcomeApplied,
		w.ownershipActor().ID, ownershipNow)
	checkRefused("zero version pin", err)
	_, err = adminPool.Exec(ctx, `
                insert into cases.case_corrections (tenant_id, case_id, correction_id, correction_no, fact_id, version_no,
                        summary_no, proposed_value, spans, outcome, actor_id, channel, reason, observed_at, received_at, created_at)
                values ($1,$2,$3,3,$4,1,0,'"Казань"'::jsonb,'[]'::jsonb,$5,$6,'operator','оператор сверил источник',$7,$7,$7)`,
		w.tenant, caseID, nextCorrectionProbeID(), factID, casesDomain.CorrectionOutcomeApplied,
		w.ownershipActor().ID, ownershipNow)
	checkRefused("zero summary link (non-null)", err)

	// The times discipline: received >= observed - 48h (the 2.2 rule).
	_, err = adminPool.Exec(ctx, `
                insert into cases.case_corrections (tenant_id, case_id, correction_id, correction_no, fact_id, version_no,
                        summary_no, proposed_value, spans, outcome, actor_id, channel, reason, observed_at, received_at, created_at)
                values ($1,$2,$3,3,$4,1,null,'"Казань"'::jsonb,'[]'::jsonb,$5,$6,'оператор сверил источник',$7,$8,$8,$8)`,
		w.tenant, caseID, nextCorrectionProbeID(), factID, casesDomain.CorrectionOutcomeApplied,
		w.ownershipActor().ID, ownershipNow.Add(48*time.Hour), ownershipNow.Add(-time.Hour))
	checkRefused("times beyond the 48h skew", err)

	// The correction number and the correction id uniqueness.
	checkDuplicate("raced correction number", w.insertCorrectionRow(t, caseID, factID, nextCorrectionProbeID(), 1, casesDomain.CorrectionOutcomeApplied, `[]`, "оператор сверил источник", nil))
	checkDuplicate("duplicate correction id", w.insertCorrectionRow(t, caseID, factID, appliedRowID, 4, casesDomain.CorrectionOutcomeApplied, `[]`, "оператор сверил источник", nil))

	// The fact-version FK: a foreign version of another fact and a
	// foreign case of the summary link.
	_, err = adminPool.Exec(ctx, `
                insert into cases.case_corrections (tenant_id, case_id, correction_id, correction_no, fact_id, version_no,
                        summary_no, proposed_value, spans, outcome, actor_id, channel, reason, observed_at, received_at, created_at)
                values ($1,$2,$3,4,$4,7,null,'"Казань"'::jsonb,'[]'::jsonb,$5,$6,'operator','оператор сверил источник',$7,$7,$7)`,
		w.tenant, caseID, nextCorrectionProbeID(), nextCorrectionProbeID(), casesDomain.CorrectionOutcomeApplied,
		w.ownershipActor().ID, ownershipNow)
	checkForeign("foreign fact version", err)
	_, err = adminPool.Exec(ctx, `
                insert into cases.case_corrections (tenant_id, case_id, correction_id, correction_no, fact_id, version_no,
                        summary_no, proposed_value, spans, outcome, actor_id, channel, reason, observed_at, received_at, created_at)
                values ($1,$2,$3,4,$4,1,7,'"Казань"'::jsonb,'[]'::jsonb,$5,$6,'operator','оператор сверил источник',$7,$7,$7)`,
		w.tenant, caseID, nextCorrectionProbeID(), factID, casesDomain.CorrectionOutcomeApplied,
		w.ownershipActor().ID, ownershipNow)
	checkForeign("foreign summary link", err)

	// The command and outbox dictionary ALTERs: the correction record
	// command word and the correction event type are the legal dictionary
	// words (the dishonest forms are refused).
	if _, err := adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'record_correction',$3,repeat('d',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("correction-dict-probe-%d", correctionSeedSeq), ownershipNow); err != nil {
		t.Fatalf("the record_correction command word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.command_records (tenant_id, actor_id, command, idempotency_key, request_hash, outcome, created_at)
                values ($1,$2,'correct_fact',$3,repeat('d',64),'{}'::jsonb,$4)`,
		w.tenant, w.ownershipActor().ID, fmt.Sprintf("correction-dict-probe-%d", correctionSeedSeq), ownershipNow)
	checkRefused("foreign command word", err)
	if _, err := adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.correction_recorded',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextCorrectionProbeID(), caseID, nextCorrectionProbeID(), nextCorrectionProbeID(), ownershipNow,
		fmt.Sprintf("correction-outbox-probe-%d", correctionSeedSeq), ownershipNow); err != nil {
		t.Fatalf("the case.correction_recorded event word must land, got %v", err)
	}
	_, err = adminPool.Exec(ctx, `
                insert into cases.outbox_events (tenant_id,event_id,event_type,schema_version,aggregate_id,source_connection_id,correlation_id,occurred_at,payload,dedup_key,created_at)
                values ($1,$2,'case.correction_reverted',1,$3,$4,$5,$6,'{}'::jsonb,$7,$8)`,
		w.tenant, nextCorrectionProbeID(), caseID, nextCorrectionProbeID(), nextCorrectionProbeID(), ownershipNow,
		fmt.Sprintf("correction-outbox-probe-%d", correctionSeedSeq), ownershipNow)
	checkRefused("foreign outbox event word", err)
}

// correctionParams builds the store-level command params of the
// correction probes (the value is canonicalized through the domain
// discipline — the digest is the material identity of the pour).
func correctionParams(scope tenancy.Context, actor id.UUID, caseID, conversationID, factID id.UUID, versionNo, summaryNo int64, rawValue []byte, spans []string, reason, key string, now time.Time) casesPort.WorkCommandParams {
	value, digest, err := casesDomain.FactValue(rawValue)
	if err != nil {
		panic(err)
	}
	return casesPort.WorkCommandParams{
		Scope: scope, Command: "record_correction", Key: key,
		RequestHash: strings.Repeat("e", 64), Actor: actor,
		CaseID: caseID, ConversationID: conversationID,
		FactID: factID, VersionNo: versionNo, SummaryNo: summaryNo,
		Value: value, ValueDigest: digest,
		Spans: spans, Reason: reason,
		ObservedAt:              now.Add(-time.Minute),
		ReceivedAt:              now,
		CorrectionID:            nextCorrectionProbeID(),
		CorrectionOutboxEventID: nextCorrectionProbeID(),
		CorrelationID:           nextCorrectionProbeID(), Outcome: []byte(`{}`), Now: now,
	}
}

// TestCorrectionsStoreRecordAndPour covers the store-level surface of
// the correction record command and the composed context snapshot: the
// target guards (the version pin, the foreign fact, the summary link),
// the domain guard under the locks (the fact value discipline, the
// byte-exact spans against the cited source, the mandatory reason, the
// 2.2 times), the matrix gate (the disputed outcome writes NO fact
// version — the system value is never rewritten), the applied pour
// through the EXISTING record discipline (the operator source, the
// correction:<id> source key — never an automatic confirmation), the
// identifiers-only outbox envelope, the command key conflict, the
// monotonic correction numbers and the ONE-transaction snapshot of the
// context material together with the correction rows.
func TestCorrectionsStoreRecordAndPour(t *testing.T) {
	w := newCasesWorld(t, 995)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-correction-store-1", "sess-correction-store-1", `{"call":"inbound-call","from":"+79000000402","say":"доставка нужна к пятнице"}`)
	conversationID := w.conversationFirst(t, caseID)
	store := casesStore.NewPostgres(runtimePool)
	scope, err := tenancy.New(w.ownershipActor().ID, w.tenant)
	if err != nil {
		t.Fatal(err)
	}
	now := ownershipNow

	// The honest fact of the case (the correction target) and the matrix
	// fact (the dispute target).
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "address", Value: []byte(`"Казань, улица Кремлёвская, 1"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-2-15-address",
		IdempotencyKey: "fact-2-15-address", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "price", Value: []byte(`"1500.00"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-2-15-price",
		IdempotencyKey: "fact-2-15-price", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	var addressFactID, priceFactID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, caseID).Scan(&addressFactID); err != nil {
		t.Fatal(err)
	}
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='price'`,
		w.tenant, caseID).Scan(&priceFactID); err != nil {
		t.Fatal(err)
	}
	// The summary of THIS case (the summary-linked correction target —
	// the direct admin row of the 2.14 probe precedent).
	summaryText := "Клиент назвал адрес доставки: Казань, улица Кремлёвская, 1. Срок доставки не назван."
	if _, err := adminPool.Exec(ctx, `
                insert into cases.case_summaries (tenant_id, case_id, summary_no, kind, actor_id, text, claims,
                        event_count, fact_version_count, owner_epoch, mode_version, created_at)
                values ($1,$2,1,$3,$4,$5,$6::jsonb,1,2,1,1,$7)`,
		w.tenant, caseID, casesDomain.SummaryKindAI, w.ownershipActor().ID, summaryText,
		honestSummaryClaims(), now); err != nil {
		t.Fatal(err)
	}

	// The target guards: the version pin out of the ladder range, the
	// foreign fact of another case, the missing summary link.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 7, 0, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-pin", now)), "Цель коррекции")
	foreignCase, foreignWork := w.acceptedCase(t, "evt-correction-store-2", "sess-correction-store-2", `{"call":"inbound-call","from":"+79000000403"}`)
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: foreignCase, WorkItemID: foreignWork,
		MaterialKey: "address", Value: []byte(`"Москва, улица Тверская, 2"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "fact-2-15-foreign",
		IdempotencyKey: "fact-2-15-foreign", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	var foreignFactID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, foreignCase).Scan(&foreignFactID); err != nil {
		t.Fatal(err)
	}
	assertRuleError(t, store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		foreignFactID, 1, 0, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-foreign", now)), "Цель коррекции")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 7, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-summary", now)), "Сводка коррекции")

	// The domain guards under the locks: the byte-exact span against the
	// cited source (the event payload), the mandatory reason, the value
	// discipline, the 2.2 times.
	assertRuleError(t, store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 0, []byte(`"Москва"`), []string{"Клиент отказался от доставки"}, "оператор сверил источник", "correction-span", now)), "байт-в-байт")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 0, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "   ", "correction-reason", now)), "причина коррекции обязательна")
	invalidValueParams := correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 0, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-value", now)
	invalidValueParams.Value = []byte("не JSON")
	assertRuleError(t, store.ExecuteWorkCommand(ctx, invalidValueParams), "JSON")
	futureParams := correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 0, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-times", now)
	futureParams.ObservedAt = now.Add(96 * time.Hour)
	assertRuleError(t, store.ExecuteWorkCommand(ctx, futureParams), "слишком далеко в будущем")
	// The summary-linked span cites the summary text (the byte-precision
	// probe of the second honest evidence source).
	assertRuleError(t, store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 1, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-summary-span", now)), "байт-в-байт")

	// The honest applied record: the correction number 1, the outcome
	// decided by the matrix gate (a non-matrix key pours), the new fact
	// version through the EXISTING record discipline.
	if err := store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 0, []byte(`"Москва, улица Тверская, 2"`), []string{"доставка нужна к пятнице"}, "оператор сверил событие", "correction-record-1", now)); err != nil {
		t.Fatalf("the honest applied correction must land, got %v", err)
	}
	var outcome string
	var pouredSourceID, pouredSourceType string
	if err := adminPool.QueryRow(ctx, `
                select outcome from cases.case_corrections
                where tenant_id=$1 and case_id=$2 and correction_no=1`,
		w.tenant, caseID).Scan(&outcome); err != nil {
		t.Fatal(err)
	}
	if outcome != casesDomain.CorrectionOutcomeApplied {
		t.Fatalf("the non-matrix key pours, got outcome %s", outcome)
	}
	if err := adminPool.QueryRow(ctx, `
                select source_id, source_type from cases.fact_versions
                where tenant_id=$1 and fact_id=$2 and version_no=2`,
		w.tenant, addressFactID).Scan(&pouredSourceID, &pouredSourceType); err != nil {
		t.Fatal(err)
	}
	if pouredSourceType != casesDomain.FactSourceOperator ||
		!strings.HasPrefix(pouredSourceID, "correction:") {
		t.Fatalf("the pour carries the operator source and the correction:<id> key, got type=%s source=%s",
			pouredSourceType, pouredSourceID)
	}
	var correctionRowID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select correction_id from cases.case_corrections
                where tenant_id=$1 and case_id=$2 and correction_no=1`,
		w.tenant, caseID).Scan(&correctionRowID); err != nil {
		t.Fatal(err)
	}
	if casesDomain.CorrectionFactSourceID(correctionRowID) != pouredSourceID {
		t.Fatalf("the pour source key composes from the correction row id, got %s want %s",
			pouredSourceID, casesDomain.CorrectionFactSourceID(correctionRowID))
	}

	// The pour is NEVER an automatic confirmation: two distinct digests
	// without a confirmation derive the honest conflicting word (the
	// resolution is the existing confirm_fact_version of a human).
	material, err := store.ReadFactMaterial(ctx, scope, caseID, addressFactID)
	if err != nil {
		t.Fatal(err)
	}
	status, _ := casesDomain.DeriveFactStatus(material.Versions, material.Confirmations)
	if status != casesDomain.FactStatusConflicting {
		t.Fatalf("the poured fact derives the honest conflicting word, got %s", status)
	}

	// The disputed record: the matrix field NEVER pours — the outcome is
	// the server decision, the fact version count stays, the proposed
	// value survives as the dispute row.
	if err := store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		priceFactID, 1, 0, []byte(`"1200.00"`), []string{"доставка нужна к пятнице"}, "клиент называл другую цену", "correction-record-2", now)); err != nil {
		t.Fatalf("the honest disputed correction must land, got %v", err)
	}
	if err := adminPool.QueryRow(ctx, `
                select outcome from cases.case_corrections
                where tenant_id=$1 and case_id=$2 and correction_no=2`,
		w.tenant, caseID).Scan(&outcome); err != nil {
		t.Fatal(err)
	}
	if outcome != casesDomain.CorrectionOutcomeDisputed {
		t.Fatalf("the matrix field disputes, got outcome %s", outcome)
	}
	var priceVersions int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.fact_versions where tenant_id=$1 and fact_id=$2`,
		w.tenant, priceFactID).Scan(&priceVersions); err != nil {
		t.Fatal(err)
	}
	if priceVersions != 1 {
		t.Fatalf("the dispute writes NO fact version (the system value is never rewritten), got %d versions", priceVersions)
	}

	// The summary-linked honest record: the span cites the summary text.
	if err := store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 2, 1, []byte(`"Москва, улица Тверская, 2"`), []string{"Срок доставки не назван."}, "оператор сверил сводку", "correction-record-3", now)); err != nil {
		t.Fatalf("the summary-linked correction with the summary span must land, got %v", err)
	}

	// The identifiers-only outbox envelope: the proposed value and the
	// spans NEVER enter (the perimeter of the outbox).
	var envelopePayload []byte
	if err := adminPool.QueryRow(ctx, `
                select payload from cases.outbox_events
                where tenant_id=$1 and event_type='case.correction_recorded'
                order by created_at, event_id limit 1`,
		w.tenant).Scan(&envelopePayload); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(envelopePayload, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tenant_id", "case_id", "correction_id", "correction_no", "fact_id", "version_no", "summary_no", "outcome", "span_count", "actor_id", "occurred_at"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("the envelope carries the identifier/word key %s, got %s", key, envelopePayload)
		}
	}
	if strings.Contains(string(envelopePayload), "Тверская") || strings.Contains(string(envelopePayload), "доставка нужна к пятнице") {
		t.Fatalf("the proposed value and the spans NEVER enter the envelope, got %s", envelopePayload)
	}

	// The command key conflict: the same key with ANOTHER material is the
	// honest conflict (the idempotency surface of 2.2..2.14 — the exec
	// rolls back, no residue lands).
	keyConflict := store.ExecuteWorkCommand(ctx, correctionParams(scope, w.ownershipActor().ID, caseID, conversationID,
		addressFactID, 1, 0, []byte(`"Другое значение"`), []string{"доставка нужна к пятнице"}, "иной материал", "correction-record-1", now))
	if !errors.Is(keyConflict, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material is the command key conflict, got %v", keyConflict)
	}
	var correctionCount int
	if err := adminPool.QueryRow(ctx, `
                select count(*) from cases.case_corrections where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID).Scan(&correctionCount); err != nil {
		t.Fatal(err)
	}
	if correctionCount != 3 {
		t.Fatalf("the rolled-back conflict leaves no residue, got %d rows", correctionCount)
	}

	// The ONE-transaction snapshot: the context material together with
	// the correction rows (the consistent read of the composed view).
	snapshot, err := store.ReadDecisionContextWithCorrections(ctx, scope, caseID, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Corrections) != 3 {
		t.Fatalf("the snapshot reads the correction rows, got %d", len(snapshot.Corrections))
	}
	if snapshot.Corrections[0].CorrectionNo != 1 || snapshot.Corrections[0].Outcome != casesDomain.CorrectionOutcomeApplied {
		t.Fatalf("the rows are ordered by the correction numbers, got %+v", snapshot.Corrections[0])
	}
	if snapshot.Corrections[2].SummaryNo != 1 || snapshot.Corrections[2].Spans[0] != "Срок доставки не назван." {
		t.Fatalf("the summary link and the spans travel the snapshot, got %+v", snapshot.Corrections[2])
	}
	if snapshot.Input.Watermark.FactVersionCount != 4 {
		t.Fatalf("the snapshot watermark counts every fact version (2 facts + 2 pours), got %d", snapshot.Input.Watermark.FactVersionCount)
	}
	// The per-fact derivation over the snapshot material: the LAST
	// correction of the address fact speaks (the markers appear with the
	// material only).
	addressCorrections := []casesDomain.CorrectionRecord{}
	for _, record := range snapshot.Corrections {
		if record.FactID == addressFactID {
			addressCorrections = append(addressCorrections, record)
		}
	}
	view := casesDomain.DeriveCorrectionState(addressCorrections)
	if !view.HasCorrection || view.CorrectionNo != 3 || view.SourceKind != casesDomain.CorrectionSourceSummary {
		t.Fatalf("the LAST correction of the fact speaks with the summary source kind, got %+v", view)
	}
	if view.CorrectionMarkerText() != "исправлено" {
		t.Fatalf("the applied marker renders, got %q", view.CorrectionMarkerText())
	}
}

// TestCorrectionsAppCommandAndView covers the app-level surface of the
// Story 2.15 correction command and the context view markers: the scope
// gate BEFORE the material guards (the audit-only member never writes the
// operator act — the invalid material never masks the refusal), the
// domain guards of the operator material (the honest 400 before SQL),
// the whole-material replay and the key conflict, the honest
// target-foreign words of the pre-read, the applied pour through the
// command contour (the outcome word of the matrix gate travels the stored
// outcome, the new fact version pours with the correction:<id> source
// key, the read-time status flips conflicting, the LIVE staleness of the
// 2.14 summary grows with the fact-version counter), the disputed matrix
// field (NO fact version — the system value is never rewritten) and the
// fact-row markers of the decision context view (the control branch: a
// case without corrections carries the ZERO correction state — the
// markers appear only with the material).
func TestCorrectionsAppCommandAndView(t *testing.T) {
	w := newCasesWorld(t, 967)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-correction-app-1", "sess-correction-app-1", `{"call":"inbound-call","from":"+79000000404","say":"доставка нужна к пятнице"}`)
	now := ownershipNow

	// The confirmed target fact (the channel-event source with the
	// confirmation — the applied correction flips it conflicting
	// read-time) and the matrix fact (the dispute target of the price).
	w.recordConfirmedFact(t, caseID, workItemID, "address", `"Казань, Кремлёвская 1"`, "evt-correction-app-1", "correction-app-1")
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: "price", Value: []byte(`"1500.00"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "correction-app-price-src",
		Reason: "цена каталога", IdempotencyKey: "correction-app-price", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	var addressFactID, priceFactID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, caseID).Scan(&addressFactID); err != nil {
		t.Fatal(err)
	}
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='price'`,
		w.tenant, caseID).Scan(&priceFactID); err != nil {
		t.Fatal(err)
	}
	// The summary of THIS case (the context material — the markers derive
	// over the consistent snapshot; the watermark pins fact_version_count
	// of the pre-correction truth: the address v1 + the price v1).
	summaryText := "Клиент назвал адрес доставки: Казань, Кремлёвская 1."
	if _, err := adminPool.Exec(ctx, `
                insert into cases.case_summaries (tenant_id, case_id, summary_no, kind, actor_id, text, claims,
                        event_count, fact_version_count, owner_epoch, mode_version, created_at)
                values ($1,$2,1,$3,$4,$5,$6::jsonb,1,2,1,1,$7)`,
		w.tenant, caseID, casesDomain.SummaryKindAI, w.ownershipActor().ID, summaryText,
		honestSummaryClaims(), now); err != nil {
		t.Fatal(err)
	}

	// The view shorthand with the row finder over the three fact groups.
	contextOf := func() casesAPI.DecisionContextView {
		view, err := w.cases.DecisionContext(ctx, w.ownershipActor(), w.tenant, caseID)
		if err != nil {
			t.Fatal(err)
		}
		return view
	}
	findRow := func(view casesAPI.DecisionContextView, factID id.UUID) *casesAPI.ContextFactRow {
		for i := range view.Facts.Confirmed {
			if view.Facts.Confirmed[i].FactID == factID.String() {
				return &view.Facts.Confirmed[i]
			}
		}
		for i := range view.Facts.Unconfirmed {
			if view.Facts.Unconfirmed[i].FactID == factID.String() {
				return &view.Facts.Unconfirmed[i]
			}
		}
		for i := range view.Facts.Conflicting {
			if view.Facts.Conflicting[i].FactID == factID.String() {
				return &view.Facts.Conflicting[i]
			}
		}
		return nil
	}
	// The command shorthand.
	correct := func(actor identityDomain.Principal, factID id.UUID, versionNo int64, rawValue []byte, spans []string, reason, key string) (casesAPI.WorkOutcome, error) {
		return w.cases.RecordCorrection(ctx, casesAPI.RecordCorrectionCommand{
			Actor: actor, TenantID: w.tenant, CaseID: caseID, FactID: factID,
			VersionNo: versionNo, SummaryNo: 0, Value: rawValue, Spans: spans, Reason: reason,
			IdempotencyKey: key, Now: now,
		})
	}

	// The control branch FIRST (before any correction): the fact rows
	// carry the ZERO correction state — the markers appear only with the
	// material (the byte-identical render of the 2.14 page).
	view := contextOf()
	if !view.Present {
		t.Fatalf("the recorded summary makes the section, got %+v", view)
	}
	if row := findRow(view, addressFactID); row == nil || row.HasCorrection || row.CorrectionMarkerText != "" ||
		row.CorrectionSourceText != "" || row.CorrectionNo != 0 || row.CorrectionOutcome != "" || row.CorrectionSpans != nil {
		state := casesAPI.ContextFactRow{}
		if row != nil {
			state = *row
		}
		t.Fatalf("the fact row without corrections carries the ZERO correction state, got %+v", state)
	}

	// The scope gate BEFORE the material guards (the M1 precedent): the
	// audit-only member is forbidden with the scope words — the invalid
	// material never masks the refusal.
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "correction-app-audit@example.org", "expert_qa",
		[]identityDomain.Scope{identityDomain.Audit}, "000000009961", "000000009962")
	auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()
	if _, err := correct(auditOnly, addressFactID, 1, []byte("не JSON"), []string{"доставка нужна к пятнице"}, "   ", "correction-app-audit-1"); !errors.Is(err, identityDomain.ErrForbidden) ||
		!strings.Contains(err.Error(), "review или configuration") {
		t.Fatalf("the audit-only member is forbidden BEFORE the material guards, got %v", err)
	}

	// The domain guards of the operator material (the honest 400 before
	// SQL): the value discipline, the mandatory reason.
	_, err := correct(w.ownershipActor(), addressFactID, 1, []byte("не JSON"), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-value-1")
	assertRuleError(t, err, "JSON")
	_, err = correct(w.ownershipActor(), addressFactID, 1, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "   ", "correction-app-reason-1")
	assertRuleError(t, err, "причина коррекции обязательна")

	// The honest target-foreign words of the pre-read: the fact of another
	// case (the same tenant) and the version pin out of the ladder.
	foreignCase, foreignWork := w.acceptedCase(t, "evt-correction-app-2", "sess-correction-app-2", `{"call":"inbound-call","from":"+79000000405"}`)
	if _, err := w.cases.RecordFact(ctx, casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: foreignCase, WorkItemID: foreignWork,
		MaterialKey: "address", Value: []byte(`"Москва, Тверская 2"`),
		SourceType: casesDomain.FactSourceOperator, SourceID: "correction-app-foreign-src",
		Reason: "адрес клиента", IdempotencyKey: "correction-app-foreign-fact", Now: now,
	}); err != nil {
		t.Fatal(err)
	}
	var foreignFactID id.UUID
	if err := adminPool.QueryRow(ctx, `
                select fact_id from cases.facts where tenant_id=$1 and case_id=$2 and material_key='address'`,
		w.tenant, foreignCase).Scan(&foreignFactID); err != nil {
		t.Fatal(err)
	}
	_, err = correct(w.ownershipActor(), foreignFactID, 1, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-foreign-1")
	assertRuleError(t, err, "Цель коррекции")
	_, err = correct(w.ownershipActor(), addressFactID, 9, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-pin-1")
	assertRuleError(t, err, "Цель коррекции")

	// The applied correction: the outcome word of the matrix gate travels
	// the stored outcome with the span counter; the pour writes the NEW
	// version through the existing record discipline (the operator source,
	// the correction:<id> source key) and the read-time status flips
	// conflicting — NEVER an automatic confirmation.
	first, err := correct(w.ownershipActor(), addressFactID, 1, []byte(`"Москва, Тверская 2"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-applied-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Command != "record_correction" || first.CorrectionOutcome != casesDomain.CorrectionOutcomeApplied || first.CorrectionSpans != 1 || !first.CorrectionID.Valid() {
		t.Fatalf("the applied outcome word travels the stored outcome, got %+v", first)
	}
	facts, err := w.cases.FactsView(ctx, w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts.Facts {
		if fact.FactID != addressFactID {
			continue
		}
		if len(fact.Versions) != 2 {
			t.Fatalf("the applied pour writes the new version, got %d", len(fact.Versions))
		}
		poured := fact.Versions[1]
		if poured.SourceType != casesDomain.FactSourceOperator || poured.SourceID != casesDomain.CorrectionFactSourceID(first.CorrectionID) {
			t.Fatalf("the pour carries the operator source with the correction key, got %s %s", poured.SourceType, poured.SourceID)
		}
		if fact.Status != casesDomain.FactStatusConflicting {
			t.Fatalf("the applied pour stays UNCONFIRMED — the read-time conflict, got %s", fact.Status)
		}
	}

	// The view markers: the fact row carries the applied marker with the
	// number, the outcome word, the time, the spans and the reason; the
	// LIVE staleness of the 2.14 summary grows with the fact-version
	// counter (the honest watermark — never a stored status).
	view = contextOf()
	row := findRow(view, addressFactID)
	if row == nil || !row.HasCorrection || row.CorrectionMarkerText != "исправлено" ||
		row.CorrectionSourceText != "источник исправления: событие" ||
		row.CorrectionNo != 1 || row.CorrectionOutcome != casesDomain.CorrectionOutcomeApplied ||
		len(row.CorrectionSpans) != 1 || row.CorrectionSpans[0] != "доставка нужна к пятнице" || row.CorrectionReason != "оператор сверил источник" {
		state := "nil"
		if row != nil {
			state = fmt.Sprintf("%+v", *row)
		}
		t.Fatalf("the fact row carries the applied marker with the material, got %s", state)
	}
	if view.Summary.Staleness != casesDomain.ContextStalenessStale || !strings.Contains(view.Summary.StalenessCountersText, "новых версий фактов: 1") {
		t.Fatalf("the applied pour grows the LIVE watermark of the summary, got %s %q", view.Summary.Staleness, view.Summary.StalenessCountersText)
	}

	// The replay: the exact repeat of the material returns the stored
	// outcome (the whole-material hash — the spans and the value
	// INCLUDED).
	replayed, err := correct(w.ownershipActor(), addressFactID, 1, []byte(`"Москва, Тверская 2"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-applied-1")
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replay || replayed.CorrectionID != first.CorrectionID || replayed.CorrectionOutcome != casesDomain.CorrectionOutcomeApplied {
		t.Fatalf("the exact repeat replays the stored outcome, got %+v", replayed)
	}
	// The key conflict: the same key with another material.
	_, err = correct(w.ownershipActor(), addressFactID, 1, []byte(`"Москва, Тверская 3"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-applied-1")
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("the same key with another material is the honest key conflict, got %v", err)
	}

	// The disputed matrix field: the price NEVER pours (the Owning System
	// value is never rewritten); the row carries the honest dispute marker
	// with the unchanged boundary words.
	disputed, err := correct(w.ownershipActor(), priceFactID, 1, []byte(`"1200.00"`), []string{"доставка нужна к пятнице"}, "клиент называет другую цену", "correction-app-disputed-1")
	if err != nil {
		t.Fatal(err)
	}
	if disputed.CorrectionOutcome != casesDomain.CorrectionOutcomeDisputed {
		t.Fatalf("the matrix field disputes — the outcome is the SERVER decision, got %+v", disputed)
	}
	facts, err = w.cases.FactsView(ctx, w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts.Facts {
		if fact.FactID == priceFactID && len(fact.Versions) != 1 {
			t.Fatalf("the disputed matrix field writes NO fact version, got %d", len(fact.Versions))
		}
	}
	view = contextOf()
	if row := findRow(view, priceFactID); row == nil || !row.HasCorrection ||
		row.CorrectionMarkerText != "оспорено (значение системы не изменено)" || row.CorrectionNo != 2 ||
		row.CorrectionOutcome != casesDomain.CorrectionOutcomeDisputed {
		state := "nil"
		if row != nil {
			state = fmt.Sprintf("%+v", *row)
		}
		t.Fatalf("the price row carries the honest dispute marker, got %s", state)
	}

	// The dispute NEVER grows the LIVE watermark (the pour never
	// happened — the Owning System value did not move; the summary
	// counters stay at the SAME honest numbers: the dispute is not a
	// rewrite, the summary stays honest about what actually changed).
	if !strings.Contains(view.Summary.StalenessCountersText, "новых версий фактов: 1") {
		t.Fatalf("the dispute never grows the watermark of the summary, got %q", view.Summary.StalenessCountersText)
	}

	// The second correction of the address fact through the SUMMARY link:
	// the last correction speaks — the summary source kind with the
	// byte-exact span of the summary text (the store guard re-validates
	// the cited source).
	second, err := w.cases.RecordCorrection(ctx, casesAPI.RecordCorrectionCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, FactID: addressFactID,
		VersionNo: 2, SummaryNo: 1, Value: []byte(`"Москва, Тверская 3"`),
		Spans: []string{"Клиент назвал адрес доставки"}, Reason: "уточнение по сводке",
		IdempotencyKey: "correction-app-applied-2", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.CorrectionOutcome != casesDomain.CorrectionOutcomeApplied {
		t.Fatalf("the non-matrix key pours again, got %+v", second)
	}
	view = contextOf()
	row = findRow(view, addressFactID)
	if row == nil || row.CorrectionNo != 3 || row.CorrectionSourceText != "источник исправления: сводка" ||
		row.CorrectionMarkerText != "исправлено" || len(row.CorrectionSpans) != 1 || row.CorrectionSpans[0] != "Клиент назвал адрес доставки" {
		state := "nil"
		if row != nil {
			state = fmt.Sprintf("%+v", *row)
		}
		t.Fatalf("the LAST correction of the fact speaks with the summary source, got %s", state)
	}
	// The pour grew the LIVE watermark again (three versions of the
	// address fact + the price v1 against the snapshot of two).
	if !strings.Contains(view.Summary.StalenessCountersText, "новых версий фактов: 2") {
		t.Fatalf("the second pour grows the LIVE watermark again, got %q", view.Summary.StalenessCountersText)
	}

	// Row 44 — the human resolution of the poured conflict: the
	// operator confirms the EXACT poured version through the EXISTING
	// confirm ladder (the live command separation of AC-2: the machine
	// proposes the version, the human confirms — never an automatic
	// confirmation). The status flips confirmed read-time, the marker
	// STAYS (the correction history never disappears — append-only),
	// the watermark stays 2 (a confirmation is not a new version).
	confirmed, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: addressFactID,
		VersionNo: 3, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		EvidenceEventID: eventIDOf(t, w, caseID, "evt-correction-app-1"),
		IdempotencyKey:  "correction-app-confirm-1", Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.FactStatus != casesDomain.FactStatusConfirmed {
		t.Fatalf("the confirm of the exact poured version resolves the conflict, got %+v", confirmed)
	}
	view = contextOf()
	row = findRow(view, addressFactID)
	if row == nil || row.Status != casesDomain.ContextFactConfirmed ||
		!strings.HasPrefix(row.StatusText, "Подтверждено: точная версия подтверждена") ||
		!row.HasCorrection || row.CorrectionNo != 3 || row.CorrectionMarkerText != "исправлено" ||
		row.CorrectionOutcome != casesDomain.CorrectionOutcomeApplied {
		state := "nil"
		if row != nil {
			state = fmt.Sprintf("%+v", *row)
		}
		t.Fatalf("the confirmed row KEEPS the correction marker (the history never disappears), got %s", state)
	}
	if !strings.Contains(view.Summary.StalenessCountersText, "новых версий фактов: 2") {
		t.Fatalf("the confirmation is not a new version — the watermark stays, got %q", view.Summary.StalenessCountersText)
	}

	// Row 52 — the optimistic concurrency of ONE pinned version: TWO
	// different corrections of the SAME confirmed version (the different
	// keys, the different values) — both rows land, both versions pour
	// (N+1, N+2), the three-value conflict is read-time (the silent
	// rewrite never exists — append-only), the LAST correction speaks.
	racyA, err := correct(w.ownershipActor(), addressFactID, 3, []byte(`"Москва, Тверская 4"`), []string{"доставка нужна к пятнице"}, "первая параллельная коррекция", "correction-app-racy-1")
	if err != nil {
		t.Fatal(err)
	}
	racyB, err := correct(w.ownershipActor(), addressFactID, 3, []byte(`"Москва, Тверская 5"`), []string{"доставка нужна к пятнице"}, "вторая параллельная коррекция", "correction-app-racy-2")
	if err != nil {
		t.Fatal(err)
	}
	if racyA.CorrectionID == racyB.CorrectionID || racyA.CorrectionOutcome != casesDomain.CorrectionOutcomeApplied ||
		racyB.CorrectionOutcome != casesDomain.CorrectionOutcomeApplied {
		t.Fatalf("the two corrections of one version are TWO rows (never a silent rewrite), got %+v %+v", racyA, racyB)
	}
	facts, err = w.cases.FactsView(ctx, w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts.Facts {
		if fact.FactID != addressFactID {
			continue
		}
		if len(fact.Versions) != 5 {
			t.Fatalf("the two corrections of one version pour BOTH versions (N+1, N+2), got %d", len(fact.Versions))
		}
		if fact.Status != casesDomain.FactStatusConflicting {
			t.Fatalf("the three-value conflict is read-time (the last poured value is not the confirmed one), got %s", fact.Status)
		}
	}
	view = contextOf()
	row = findRow(view, addressFactID)
	if row == nil || row.Status != casesDomain.ContextFactConflicting ||
		!strings.HasPrefix(row.StatusText, "Противоречие: значения расходятся") ||
		row.CorrectionNo != 5 || row.CorrectionMarkerText != "исправлено" {
		state := "nil"
		if row != nil {
			state = fmt.Sprintf("%+v", *row)
		}
		t.Fatalf("the LAST correction of the SAME version speaks over the read-time conflict, got %s", state)
	}
	if !strings.Contains(view.Summary.StalenessCountersText, "новых версий фактов: 4") {
		t.Fatalf("the both pours grow the LIVE watermark, got %q", view.Summary.StalenessCountersText)
	}

	// Row 54 — the gates of the slice: the member of another tenant and
	// the non-member read the plain 404 (the RLS regression of 2.1) —
	// the gate fires BEFORE the material guards (the honest 404 of the
	// path, never the 403 of the scope).
	foreignTenant := createForeignTenant(t, w)
	foreign := inviteMember(t, w.identity, w.owner, foreignTenant, "correction-app-foreign-tenant@example.org", "expert_qa",
		[]identityDomain.Scope{identityDomain.Review}, "000000009971", "000000009972")
	if _, err := correct(foreign, addressFactID, 1, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-foreign-tenant-1"); !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the member of another tenant never records, got %v", err)
	}
	nonMember := actorPrincipal{principalID: nextCorrectionProbeID(), stepUp: true}.principal()
	if _, err := correct(nonMember, addressFactID, 1, []byte(`"Москва"`), []string{"доставка нужна к пятнице"}, "оператор сверил источник", "correction-app-nonmember-1"); !errors.Is(err, identityDomain.ErrNotFound) && !errors.Is(err, casesDomain.ErrNotFound) {
		t.Fatalf("the non-member never records, got %v", err)
	}

	// The terminal boundary of the state dictionary: the slice carries
	// the open word only (the honest CHECK refusal — the 2.14 row-19
	// precedent; the app refusal mirror stays for the future dictionary
	// growth).
	if _, err := adminPool.Exec(ctx, `
                update cases.cases set state='resolved' where tenant_id=$1 and case_id=$2`,
		w.tenant, caseID); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("the terminal state word is impossible in this slice (the CHECK refusal), got %v", err)
		}
	}
}
