//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/staffy-co/staffy/internal/adapters/ingresssyn"
	"github.com/staffy-co/staffy/internal/adapters/observationsyn"
	"github.com/staffy-co/staffy/internal/adapters/verificationsyn"
	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesApp "github.com/staffy-co/staffy/internal/cases/app"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	casesStore "github.com/staffy-co/staffy/internal/cases/store"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
	"github.com/jackc/pgx/v5/pgconn"
)

// The Story 2.7 matrix: the State/Authority Matrix v1 reads, the
// idempotency and the provider versions, the honest failure rows without
// a value, the freshness derivation, the per-field isolation, the
// conflict badge against the confirmed facts, the rights gates and the
// append-only discipline — over the real world of the cases contour.

// observationView reads the owning-system section of the case view.
func (w *casesWorld) observationView(t *testing.T, caseID id.UUID) casesAPI.ObservationView {
	t.Helper()
	detail, err := w.cases.CaseView(context.Background(), w.ownershipActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	return detail.Observations
}

// observationSectionOf resolves the Work Item section of the observation
// view (the per-item card block of the panel).
func observationSectionOf(t *testing.T, view casesAPI.ObservationView, workItemID id.UUID) casesAPI.ObservationWorkItemView {
	t.Helper()
	for _, section := range view.Items {
		if section.WorkItemID == workItemID {
			return section
		}
	}
	t.Fatalf("the observation section of the work item %s not found", workItemID)
	return casesAPI.ObservationWorkItemView{}
}

// observationCardOf resolves one material field card of a section.
func observationCardOf(t *testing.T, section casesAPI.ObservationWorkItemView, materialKey string) casesAPI.ObservationCard {
	t.Helper()
	for _, card := range section.Fields {
		if card.MaterialKey == materialKey {
			return card
		}
	}
	t.Fatalf("the observation card of the field %s not found", materialKey)
	return casesAPI.ObservationCard{}
}

// observationRead is the read shorthand of the tests (the honest error
// return — the refusals are the matrix rows).
func (w *casesWorld) observationRead(t *testing.T, command casesAPI.ReadOwningSystemCommand) (casesAPI.WorkOutcome, error) {
	t.Helper()
	return w.cases.ReadOwningSystem(context.Background(), command)
}

// observationReadOK is the read shorthand that must pass.
func (w *casesWorld) observationReadOK(t *testing.T, command casesAPI.ReadOwningSystemCommand) casesAPI.WorkOutcome {
	t.Helper()
	outcome, err := w.cases.ReadOwningSystem(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// observationReadCommand builds the read command of the tests over the
// world owner actor.
func (w *casesWorld) observationReadCommand(caseID, workItemID id.UUID, materialKey, key string) casesAPI.ReadOwningSystemCommand {
	return casesAPI.ReadOwningSystemCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: materialKey, IdempotencyKey: key, Now: ownershipNow,
	}
}

// TestObservationDemoPathReadReplaySupersede covers the owner-approved
// demo chain: the first read lands the ok observation with the value, the
// carrier words and the provider version (№1); the exact repeat replays
// the stored outcome without a new row; a fresh key reads again — the
// provider version moves forward (v2), the observation number is the
// monotonic №2, the №1 row stays readable for the audit and the CURRENT
// observation is №2 (the re-read supersedes by a NEW row, never by an
// update).
func TestObservationDemoPathReadReplaySupersede(t *testing.T) {
	w := newCasesWorld(t, 780)
	caseID, workItemID := w.acceptedCase(t, "evt-observation-1", "sess-observation-1", `{"call":"inbound-call","from":"+79000000100"}`)

	// The first read: the ok observation with the value and the carrier
	// words (the successful read always names its carrier — AC-2).
	outcome := w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-read-1"))
	if outcome.Replay || !outcome.ObservationID.Valid() || outcome.ObservationStatus != casesDomain.ObservationStatusOk ||
		outcome.ObservationVersion != "v1" || outcome.ObservationExternalID == "" {
		t.Fatalf("expected the fresh ok observation with the carrier words, got %+v", outcome)
	}

	// The exact repeat replays the stored outcome; no new row exists.
	replay, err := w.observationRead(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-read-1"))
	if err != nil || !replay.Replay || replay.ObservationID != outcome.ObservationID {
		t.Fatalf("expected the replayed read, got %+v %v", replay, err)
	}
	view := w.observationView(t, caseID)
	if section := observationSectionOf(t, view, workItemID); section.ObservationCount != 1 {
		t.Fatalf("the replay must not write: expected 1 observation, got %d", section.ObservationCount)
	}

	// The fresh key reads again: the provider version moves forward (the
	// counter of the successful reads), the number is №2 and №1 stays
	// readable (the audit trail — AC-8).
	second := w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-read-2"))
	if second.Replay || second.ObservationVersion != "v2" || second.ObservationID == outcome.ObservationID {
		t.Fatalf("expected the new provider version as the new row, got %+v", second)
	}
	view = w.observationView(t, caseID)
	section := observationSectionOf(t, view, workItemID)
	if section.ObservationCount != 2 {
		t.Fatalf("expected 2 observations, got %d", section.ObservationCount)
	}
	card := observationCardOf(t, section, casesDomain.MaterialFieldPrice)
	if card.Current == nil || card.Current.ObservationNo != 2 || card.Current.ExternalVersion != "v2" {
		t.Fatalf("the CURRENT observation must be №2 v2, got %+v", card.Current)
	}
	if string(card.Current.Value) == "" || card.Current.ExternalID == "" || card.Current.ObservedAt.IsZero() {
		t.Fatalf("the ok observation carries the value and the carrier words, got %+v", card.Current)
	}
}

// TestObservationFreshnessDerivation covers the read-time freshness
// derivation (AC-3): the ok reads of all three matrix fields age into the
// stale state past their matrix windows — the panel state words move from
// fresh to stale with the same rows (nothing is stored; the derivation
// reads the provider-observed time against the matrix window).
func TestObservationFreshnessDerivation(t *testing.T) {
	w := newCasesWorld(t, 781)
	caseID, workItemID := w.acceptedCase(t, "evt-observation-2", "sess-observation-2", `{"call":"inbound-call","from":"+79000000101"}`)

	// Read all three matrix fields at the command moment (the fresh
	// values of the synthetic contour).
	for i, field := range casesDomain.MaterialFields {
		w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, field, "observation-fresh-"+string(rune('a'+i))))
	}

	// The view of the SAME moment: every field is fresh (the age is zero
	// — the derivation clock equals the read moment).
	w.cases = w.buildCasesObservationService(newPrivacyIDs(7811), fixedClock{ownershipNow}, nil)
	view := w.observationView(t, caseID)
	section := observationSectionOf(t, view, workItemID)
	for _, field := range casesDomain.MaterialFields {
		if card := observationCardOf(t, section, field); card.State != casesDomain.ObservationStateFresh {
			t.Fatalf("the field %s must be fresh at the read moment, got %s", field, card.State)
		}
	}

	// The view ten minutes later: every field is stale (the shortest
	// window of the matrix is 60 seconds; the stale state carries the
	// re-read word — never the fresh state).
	w.cases = w.buildCasesObservationService(newPrivacyIDs(7812), fixedClock{ownershipNow.Add(10 * time.Minute)}, nil)
	view = w.observationView(t, caseID)
	section = observationSectionOf(t, view, workItemID)
	for _, field := range casesDomain.MaterialFields {
		card := observationCardOf(t, section, field)
		if card.State != casesDomain.ObservationStateStale {
			t.Fatalf("the field %s must be stale ten minutes later, got %s", field, card.State)
		}
	}
}

// TestObservationFailureScenariosHonest covers the honest failure rows
// (AC-6): the provider timeout and the malformed answer land as FAILED
// with the reason word; the missing scope lands as UNKNOWN (the value was
// never claimable); none of the failure rows carries a value — the
// assumed value never appears as a class; the view states are honest.
func TestObservationFailureScenariosHonest(t *testing.T) {
	w := newCasesWorld(t, 782)
	caseID, workItemID := w.acceptedCase(t, "evt-observation-3", "sess-observation-3", `{"call":"inbound-call","from":"+79000000102"}`)

	// The scripted contour: the price times out, the availability answers
	// the malformed body, the order status lacks the read scope.
	script := map[string]observationsyn.Scenario{
		casesDomain.MaterialFieldPrice:        {Word: observationsyn.ScenarioProviderTimeout},
		casesDomain.MaterialFieldAvailability: {Word: observationsyn.ScenarioMalformedResponse},
		casesDomain.MaterialFieldOrderStatus:  {Word: observationsyn.ScenarioInsufficientScope},
	}
	w.cases = w.buildCasesObservationService(newPrivacyIDs(7821), fixedClock{ownershipNow}, script)

	price := w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-fail-1"))
	if price.ObservationStatus != casesDomain.ObservationStatusFailed || price.ObservationReason != casesDomain.ReadReasonProviderTimeout {
		t.Fatalf("expected the failed timeout row, got %+v", price)
	}
	availability := w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldAvailability, "observation-fail-2"))
	if availability.ObservationStatus != casesDomain.ObservationStatusFailed || availability.ObservationReason != casesDomain.ReadReasonMalformedResponse {
		t.Fatalf("expected the failed malformed row, got %+v", availability)
	}
	orderStatus := w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldOrderStatus, "observation-fail-3"))
	if orderStatus.ObservationStatus != casesDomain.ObservationStatusUnknown || orderStatus.ObservationReason != casesDomain.ReadReasonInsufficientScope {
		t.Fatalf("expected the unknown scope row, got %+v", orderStatus)
	}

	// The honest view: no failure row carries a value or the carrier
	// words; the states stay failed/unknown (never fresh).
	view := w.observationView(t, caseID)
	section := observationSectionOf(t, view, workItemID)
	for field, expected := range map[string]string{
		casesDomain.MaterialFieldPrice:        casesDomain.ObservationStateFailed,
		casesDomain.MaterialFieldAvailability: casesDomain.ObservationStateFailed,
		casesDomain.MaterialFieldOrderStatus:  casesDomain.ObservationStateUnknown,
	} {
		card := observationCardOf(t, section, field)
		if card.State != expected {
			t.Fatalf("the field %s must show %s, got %s", field, expected, card.State)
		}
		if card.Current == nil || len(card.Current.Value) != 0 || card.Current.ExternalID != "" || card.Current.ExternalVersion != "" {
			t.Fatalf("the failure row of %s never carries a value or the carrier words, got %+v", field, card.Current)
		}
	}
}

// TestObservationFieldIsolation covers the per-field isolation (AC-9): the
// catalog fields stay readable while the erp system is down — the failure
// of one owning system never masks another field and no aggregate health
// word exists (the panel carries the honest per-field states only).
func TestObservationFieldIsolation(t *testing.T) {
	w := newCasesWorld(t, 783)
	caseID, workItemID := w.acceptedCase(t, "evt-observation-4", "sess-observation-4", `{"call":"inbound-call","from":"+79000000103"}`)

	// The erp system is down (the timeout); the catalog answers ok.
	script := map[string]observationsyn.Scenario{
		casesDomain.MaterialFieldOrderStatus: {Word: observationsyn.ScenarioProviderTimeout},
	}
	w.cases = w.buildCasesObservationService(newPrivacyIDs(7831), fixedClock{ownershipNow}, script)

	price := w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-iso-1"))
	if price.ObservationStatus != casesDomain.ObservationStatusOk {
		t.Fatalf("the catalog price stays readable, got %+v", price)
	}
	orderStatus := w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldOrderStatus, "observation-iso-2"))
	if orderStatus.ObservationStatus != casesDomain.ObservationStatusFailed {
		t.Fatalf("the erp field is honestly failed, got %+v", orderStatus)
	}

	// The panel shows the honest per-field states: the ok price of the
	// catalog is NOT masked by the failed order_status of the erp.
	view := w.observationView(t, caseID)
	section := observationSectionOf(t, view, workItemID)
	if card := observationCardOf(t, section, casesDomain.MaterialFieldPrice); card.State != casesDomain.ObservationStateFresh {
		t.Fatalf("the catalog price is fresh — the erp failure does not mask it, got %s", card.State)
	}
	if card := observationCardOf(t, section, casesDomain.MaterialFieldOrderStatus); card.State != casesDomain.ObservationStateFailed {
		t.Fatalf("the erp field is failed, got %s", card.State)
	}
}

// TestObservationConflictBadgeWithConfirmedFact covers the conflict badge
// (AC-5): the byte comparison of the canonical observation value with the
// confirmed exact fact of the same material key — the equal bytes show the
// equal badge, the different bytes show the honest «расхождение», the
// field without a fact shows no_fact; the resolution always stays with the
// human (no auto-resolution word exists).
func TestObservationConflictBadgeWithConfirmedFact(t *testing.T) {
	w := newCasesWorld(t, 784)
	caseID, workItemID := w.acceptedCase(t, "evt-observation-5", "sess-observation-5", `{"call":"inbound-call","from":"+79000000104"}`)

	// The confirmed price fact with the SAME canonical bytes as the
	// synthetic observation value (the equal badge basis).
	w.recordConfirmedFact(t, caseID, workItemID, casesDomain.MaterialFieldPrice, `{"price_quote":"3000"}`, "evt-observation-5", "1")

	// The ok read of the price: the badge compares the bytes — equal.
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-conflict-1"))
	// The ok read of the availability: no confirmed fact of the field
	// exists — no_fact (the comparison happens only over a read value).
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldAvailability, "observation-conflict-2"))
	view := w.observationView(t, caseID)
	section := observationSectionOf(t, view, workItemID)
	if card := observationCardOf(t, section, casesDomain.MaterialFieldPrice); card.ConflictBadge != casesDomain.ConflictBadgeEqual {
		t.Fatalf("the equal bytes show the equal badge, got %s", card.ConflictBadge)
	}
	// The availability has no fact: no_fact.
	if card := observationCardOf(t, section, casesDomain.MaterialFieldAvailability); card.ConflictBadge != casesDomain.ConflictBadgeNoFact {
		t.Fatalf("the field without a fact shows no_fact, got %s", card.ConflictBadge)
	}

	// A second confirmed fact with DIFFERENT bytes: the badge of the
	// availability read shows the honest «расхождение» (differs).
	w.recordConfirmedFact(t, caseID, workItemID, casesDomain.MaterialFieldAvailability, `{"stock_count":"99"}`, "evt-observation-5", "2")
	view = w.observationView(t, caseID)
	section = observationSectionOf(t, view, workItemID)
	if card := observationCardOf(t, section, casesDomain.MaterialFieldAvailability); card.ConflictBadge != casesDomain.ConflictBadgeDiffers {
		t.Fatalf("the different bytes show the differs badge, got %s", card.ConflictBadge)
	}
}

// TestObservationFieldNotInMatrixAndTerminal covers the honest refusals of
// the read command: the material field outside the fixed matrix is unknown
// as a class (the machine-readable field-not-in-matrix refusal with the
// field dictionary; no row is ever written); the terminal Work Item
// refuses the read (the merged item lives on in its target — the 2-2
// boundary).
func TestObservationFieldNotInMatrixAndTerminal(t *testing.T) {
	w := newCasesWorld(t, 785)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-observation-6", "sess-observation-6", `{"call":"inbound-call","from":"+79000000105"}`)

	// The field outside the matrix: unknown as a class.
	_, err := w.observationRead(t, w.observationReadCommand(caseID, workItemID, "magic_field", "observation-matrix-1"))
	var rule casesDomain.WorkRuleError
	if !errors.As(err, &rule) || rule.Rule != "field_not_in_matrix" {
		t.Fatalf("expected the field-not-in-matrix refusal, got %+v", err)
	}
	if view := w.observationView(t, caseID); observationSectionOf(t, view, workItemID).ObservationCount != 0 {
		t.Fatal("the refused field never writes a row")
	}

	// The terminal Work Item: classify, then resolve, then the honest
	// refusal of the read (the resolved state is final).
	classify := w.workBaseInput(caseID, workItemID, 1)
	classify.Reason, classify.IdempotencyKey = "Классификация", "observation-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{WorkCommandInput: classify, Type: "поиск детали", Title: "Подбор детали"}); err != nil {
		t.Fatal(err)
	}
	resolve := w.workBaseInput(caseID, workItemID, 2)
	resolve.Reason, resolve.IdempotencyKey = "Решено", "observation-resolve-1"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{WorkCommandInput: resolve, ToState: "resolved"}); err != nil {
		t.Fatal(err)
	}
	_, err = w.observationRead(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-terminal-1"))
	assertRuleError(t, err, "Терминальный элемент работы не читается")
}

// TestObservationCommandsRefuseAuditOnlyMember covers the rights gate of
// the read command (the M1 precedent of 2-5): an audit-only member reads
// the case view but never issues the owning-system reads — the refusal is
// the honest 403 forbidden word with the scope text BEFORE every material
// guard. The composite probes pin the gate order live: neither the field
// outside the matrix nor the terminal Work Item masks the scope refusal
// (the review R1 mutation basis — moving the gate after the material
// guards must redden this test), nothing is ever written.
func TestObservationCommandsRefuseAuditOnlyMember(t *testing.T) {
	w := newCasesWorld(t, 786)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-observation-7", "sess-observation-7", `{"call":"inbound-call","from":"+79000000106"}`)
	// The member ids avoid the world suffixes (the owner is seed*10+1, the
	// backup is seed*10+8): seed*10+6/9 stay free.
	auditor := inviteMember(t, w.identity, w.readinessActor(true), w.tenant, "observation-auditor-786@example.org", "expert_qa", []identityDomain.Scope{identityDomain.Audit}, "000000007866", "000000007869")
	auditOnly := actorPrincipal{principalID: auditor.ID, stepUp: true}.principal()

	read := func(field string) error {
		_, err := w.cases.ReadOwningSystem(ctx, casesAPI.ReadOwningSystemCommand{
			Actor: auditOnly, TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			MaterialKey: field, IdempotencyKey: "observation-audit-1", Now: ownershipNow,
		})
		return err
	}
	assertScopeRefusal := func(err error, probe string) {
		t.Helper()
		if !errors.Is(err, identityDomain.ErrForbidden) || !strings.Contains(err.Error(), "review или configuration") {
			t.Fatalf("%s: expected the honest 403 forbidden refusal with the scope words, got %v", probe, err)
		}
	}

	// The healthy field of the alive Work Item: the plain scope refusal.
	assertScopeRefusal(read(casesDomain.MaterialFieldPrice), "the healthy read")
	// The field outside the matrix never masks the scope words: the rights
	// gate fires FIRST (before the matrix guard).
	assertScopeRefusal(read("magic_field"), "the field outside the matrix")

	// The terminal Work Item never masks the scope words either: classify,
	// resolve, then the audit-only read still refuses with the scope text.
	classify := w.workBaseInput(caseID, workItemID, 1)
	classify.Reason, classify.IdempotencyKey = "Классификация", "observation-audit-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{WorkCommandInput: classify, Type: "поиск детали", Title: "Подбор детали"}); err != nil {
		t.Fatal(err)
	}
	resolve := w.workBaseInput(caseID, workItemID, 2)
	resolve.Reason, resolve.IdempotencyKey = "Решено", "observation-audit-resolve-1"
	if _, err := w.cases.TransitionWorkItem(ctx, casesAPI.TransitionCommand{WorkCommandInput: resolve, ToState: "resolved"}); err != nil {
		t.Fatal(err)
	}
	assertScopeRefusal(read(casesDomain.MaterialFieldPrice), "the terminal Work Item")

	// Nothing landed: the observation surface stays clean.
	view := w.observationView(t, caseID)
	if section := observationSectionOf(t, view, workItemID); section.ObservationCount != 0 {
		t.Fatalf("the audit-only member writes nothing, got %d observations", section.ObservationCount)
	}
}

// TestObservationForeignTenantIsolation covers the tenant isolation: the
// read command of the foreign tenant sees the plain 404 (the existence is
// never disclosed — the RLS + the server-side authorization).
func TestObservationForeignTenantIsolation(t *testing.T) {
	w := newCasesWorld(t, 787)
	caseID, workItemID := w.acceptedCase(t, "evt-observation-8", "sess-observation-8", `{"call":"inbound-call","from":"+79000000107"}`)

	_, err := w.cases.ReadOwningSystem(context.Background(), casesAPI.ReadOwningSystemCommand{
		Actor: w.ownershipActor(), TenantID: id.MustParse("01994f64-6c00-7000-8000-0000000000f1"),
		CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: casesDomain.MaterialFieldPrice, IdempotencyKey: "observation-foreign-1", Now: ownershipNow,
	})
	if !errors.Is(err, casesDomain.ErrNotFound) && !errors.Is(err, identityDomain.ErrNotFound) {
		t.Fatalf("expected the honest not-found of the foreign tenant, got %v", err)
	}
}

// TestObservationKeyConflictAndNilReader covers the remaining idempotency
// and wiring rows: the same command key with another material is the
// honest key conflict (the request hash covers the material field —
// AC-8); the service without the wired reader refuses BEFORE any record
// (an «ok» contour is never assumed — the 2-1 «коннектор без
// верификатора» precedent).
func TestObservationKeyConflictAndNilReader(t *testing.T) {
	w := newCasesWorld(t, 788)
	caseID, workItemID := w.acceptedCase(t, "evt-observation-9", "sess-observation-9", `{"call":"inbound-call","from":"+79000000108"}`)

	// The same key with another material field: the honest key conflict.
	w.observationReadOK(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldPrice, "observation-key-1"))
	_, err := w.observationRead(t, w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldAvailability, "observation-key-1"))
	if !errors.Is(err, casesDomain.ErrCommandKeyConflict) {
		t.Fatalf("expected the command key conflict, got %v", err)
	}

	// The service without the wired reader: the honest refusal BEFORE any
	// record (the unconfigured contour never assumes «ok»).
	ids := newPrivacyIDs(7881)
	nilReader := casesApp.New(casesStore.NewPostgres(runtimePool).WithIDs(ids), w.identity, ingresssyn.New(w.connector), fixedClock{ownershipNow}, ids).
		WithRouteFacts(w.employee).
		WithConnectionFacts(w.connector).
		WithMembers(w.identity).
		WithPackTaxonomy(w.employee).
		WithIdentityVerifier(verificationsyn.New())
	_, err = nilReader.ReadOwningSystem(context.Background(), w.observationReadCommand(caseID, workItemID, casesDomain.MaterialFieldOrderStatus, "observation-nil-1"))
	assertRuleError(t, err, "контур owning-system чтения не сконфигурирован")
	if view := w.observationView(t, caseID); observationSectionOf(t, view, workItemID).ObservationCount != 1 {
		t.Fatal("the unconfigured contour writes nothing (only the first ok read of the wired service)")
	}
}

// TestObservationTablesGrantsAndRLS asserts the append-only discipline of
// the observation surface at the database level: RLS + FORCE on the table,
// INSERT/SELECT for staffy_app and no UPDATE or DELETE (the mode/policy
// precedent of 2-5/2-6).
func TestObservationTablesGrantsAndRLS(t *testing.T) {
	if runtimePool == nil {
		t.Skip("no runtime pool")
	}
	ctx := context.Background()
	rows, err := runtimePool.Query(ctx, `
                select relname, relrowsecurity, relforcerowsecurity from pg_class
                where relnamespace = 'cases'::regnamespace
                  and relname in ('owning_system_observations')`)
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
	if state, ok := seen["owning_system_observations"]; !ok || !state[0] || !state[1] {
		t.Fatalf("owning_system_observations must carry RLS + FORCE (the append-only discipline), got %+v", state)
	}
	for _, grant := range []string{"INSERT", "SELECT", "UPDATE", "DELETE", "TRUNCATE"} {
		var allowed bool
		if err := runtimePool.QueryRow(ctx, `select has_table_privilege('staffy_app', $1, $2)`, "cases.owning_system_observations", grant).Scan(&allowed); err != nil {
			t.Fatal(err)
		}
		if (grant == "INSERT" || grant == "SELECT") != allowed {
			t.Fatalf("%s on cases.owning_system_observations: allowed=%v, want %v", grant, allowed, grant == "INSERT" || grant == "SELECT")
		}
	}
}

// TestObservationDBConstraintsRefuseDishonestRows pins the AC-6 database
// backstops of the append-only row: the assumed value never lands (the
// failed/unknown row carries NO value; the ok row carries its full carrier
// material) and the dictionary words of the row are the CHECK constraints —
// the honest shape is a database guarantee, not only a domain guard (the
// review R1 mutation probe proved the gap: the weakened CHECK did not
// redden a single test). The direct INSERTs run as the admin role behind
// the RLS curtain — the constraint backstop is the last line of the honest
// contour (the grants/RLS probe precedent).
func TestObservationDBConstraintsRefuseDishonestRows(t *testing.T) {
	if adminPool == nil {
		t.Skip("no admin pool")
	}
	w := newCasesWorld(t, 789)
	ctx := context.Background()
	caseID, workItemID := w.acceptedCase(t, "evt-observation-10", "sess-observation-10", `{"call":"inbound-call","from":"+79000000109"}`)

	// insert builds the direct INSERT of one observation row over the real
	// case + work item anchors (the owning system and the projection words
	// are the matrix literals of the price field).
	insert := func(materialKey string, no int64, status string, reason, value, externalID, externalVersion, observedAt any) error {
		_, err := adminPool.Exec(ctx, `
                        insert into cases.owning_system_observations (tenant_id, case_id, work_item_id, material_key, observation_no,
                                observation_id, owning_system, status, reason, value, projection, external_id, external_version,
                                observed_at, received_at, actor_id, correlation_id, created_at)
                        values ($1,$2,$3,$4,$5,$6,'catalog',$7,$8,$9,'price_quote',$10,$11,$12,$13,$14,$15,$16)`,
			w.tenant, caseID, workItemID, materialKey, no,
			fmt.Sprintf("00000000-0000-0000-0000-%012d", no), status, reason, value,
			externalID, externalVersion, observedAt, ownershipNow, w.ownershipActor().ID, w.ownershipActor().ID, ownershipNow)
		return err
	}
	// checkRefused asserts the honest 23514 refusal of the dishonest row (the
	// named AC-6 constraints when given).
	checkRefused := func(probe string, err error, constraint string) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s: the dishonest row must be refused, got accepted", probe)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.SQLState() != "23514" {
			t.Fatalf("%s: expected the check_violation 23514, got %v", probe, err)
		}
		if constraint != "" && pgErr.ConstraintName != constraint {
			t.Fatalf("%s: expected the constraint %s, got %s", probe, constraint, pgErr.ConstraintName)
		}
	}

	// The raw JSON text of the value parameter (the admin pool speaks the
	// simple protocol — the bytes would travel bytea-escaped, the string
	// travels as the JSON text the column parses).
	value := `{"price_quote":"3000"}`
	// The honest ok row lands: the row template is valid, so every refusal
	// below isolates exactly its dishonest part.
	if err := insert(casesDomain.MaterialFieldPrice, 1, "ok", nil, value, "catalog-item-1", "v1", ownershipNow); err != nil {
		t.Fatalf("the honest ok row must land, got %v", err)
	}
	// AC-6: the failed/unknown read never carries the assumed value.
	checkRefused("failed with a value", insert(casesDomain.MaterialFieldPrice, 2, "failed", "provider_timeout", value, "catalog-item-1", "v1", ownershipNow), "observation_failure_carries_no_value")
	checkRefused("unknown with a value", insert(casesDomain.MaterialFieldPrice, 3, "unknown", "insufficient_scope", value, "catalog-item-1", "v1", ownershipNow), "observation_failure_carries_no_value")
	// The ok read always names its carrier material.
	checkRefused("ok without a value", insert(casesDomain.MaterialFieldPrice, 4, "ok", nil, nil, "catalog-item-1", "v1", ownershipNow), "observation_ok_carries_material")
	checkRefused("ok without the external id", insert(casesDomain.MaterialFieldPrice, 5, "ok", nil, value, nil, "v1", ownershipNow), "observation_ok_carries_material")
	checkRefused("ok without the provider time", insert(casesDomain.MaterialFieldPrice, 6, "ok", nil, value, "catalog-item-1", "v1", nil), "observation_ok_carries_material")
	// The dictionary words of the row are the CHECK backstops (the fixed
	// vocabularies of the status, the reason and the material field).
	checkRefused("status outside the dictionary", insert(casesDomain.MaterialFieldPrice, 7, "maybe", nil, nil, nil, nil, nil), "")
	checkRefused("reason outside the dictionary", insert(casesDomain.MaterialFieldPrice, 8, "failed", "provider_flaky", nil, nil, nil, nil), "")
	checkRefused("field outside the matrix", insert("magic_field", 9, "ok", nil, value, "catalog-item-1", "v1", ownershipNow), "")
}
