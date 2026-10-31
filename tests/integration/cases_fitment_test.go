//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 4.4 integration matrix: the fitment evidence contour over the
// LIVE Postgres. The world of the matrix (the seed 745 — a free slot of
// the retro-3-3 register; the probe family continues the 88xxx c2xx
// family of Epic 4) seals the auto-parts@5.0.0 pin through the full live
// release chain (the pin chain 2.0.0 → 3.0.0 → 4.0.0 → the compatible
// additive upgrade 5.0.0 with the fitment policy) BEFORE the case
// ingestion. The matrix walks the record command LIVE (the guard
// refusals BEFORE any SQL with the dictionary words, the LIVE digest pins
// of both inputs, the monotonic evidence numbers, the idempotent replay),
// the read-time state ladder (the proven original and cross chains, the
// researched not-found, the supersede by the new number, the conflicted
// pair of the two sources, the ambiguous question, the stale states of
// the age and BOTH pins) and the RLS blindness. The degradation world
// (the seed 746) pins the 4.0.0 pack — the part request schema stays
// defined (the control) while the fitment panel carries the honest
// absence word.

// fitmentSeedSeq keeps the seeded probe rows globally unique.
var fitmentSeedSeq int64

// nextFitmentProbeID mints one globally unique probe id of the matrix.
func nextFitmentProbeID() id.UUID {
	fitmentSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 88200+fitmentSeedSeq))
}

// fitmentSealPackFive seals the auto-parts@5.0.0 pin through the live
// release chain: the incompatible 1.0.0→2.0.0 pin (confirmed), the
// compatible 2.0.0→3.0.0 upgrade (the identification schema), the
// compatible 3.0.0→4.0.0 upgrade (the additive part request schema), the
// compatible 4.0.0→5.0.0 upgrade (the additive fitment policy — Changed
// [fitment]), the assemble/version/seal of the new manifest, the fresh
// profile approval over the new link and the new acting admission
// decision.
func fitmentSealPackFive(t *testing.T, w *casesWorld, employee id.UUID) employeeDomain.ManifestLink {
	t.Helper()
	ctx := context.Background()
	// The four-step pin chain: 2.0.0 (confirmed incompatible) → 3.0.0
	// (compatible additive identification) → 4.0.0 (compatible additive
	// part request schema) → 5.0.0 (compatible additive fitment policy).
	current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c221"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "2.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(1)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	current, err = w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c222"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "3.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(2)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	current, err = w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c223"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "4.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(2)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	current, err = w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c224"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "5.0.0",
		Variants: map[string]any{"parts_interest": "both", "matching_depth": float64(2)}, Confirmed: true,
	}); err != nil {
		t.Fatal(err)
	}
	// The release chain over the upgraded refs (the live revision walk).
	view, err := w.employee.ReleaseView(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	revision := int64(0)
	if view.Draft != nil {
		revision = view.Draft.Revision
	}
	draft := w.assemble(t, employee, revision)
	version := w.createVersion(t, employee, draft.Revision, "Пин 5.0.0: политика применимости")
	manifest, err := w.seal(t, employee, version.Version, draft.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	link := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
	// The fresh profile approval over the new link (the 5.0.0 pack carries
	// the same journeys as 4.0.0 — no new coverage cells).
	content := profileContent(w.owner.ID, w.backup)
	content.Matrix = append(content.Matrix,
		employeeDomain.CoverageCell{
			Journey: "идентификация автомобиля", Route: employeeDomain.Route{Channel: "phone", Direction: employeeDomain.DirectionInbound},
			Modality: employeeDomain.ModalityVoice, ConversationMode: "sales", AutonomyClass: "assisted",
			EvalRef: "eval://auto-parts/identify/voice-v1", HumanFallback: "human_callback по регламенту",
		},
		employeeDomain.CoverageCell{
			Journey: "идентификация автомобиля", Route: employeeDomain.Route{Channel: "web_chat", Direction: employeeDomain.DirectionInbound},
			Modality: employeeDomain.ModalityText, ConversationMode: "sales", AutonomyClass: "assisted",
			EvalRef: "eval://auto-parts/identify/text-v1", HumanFallback: "human_callback по регламенту",
		},
	)
	actor := actorPrincipal{principalID: w.owner.ID, stepUp: true}
	if _, err := w.saveDraft(t, employee, link, content, 2, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := w.approve(t, employee, 3, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.PublishAdmission(ctx, employeeAPI.PublishAdmissionCommand{
		Actor: actor.principal(), TenantID: w.tenant, EmployeeID: employee,
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000c225"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
	return link
}

// fitmentRecord records one evidence through the LIVE command (the
// operator-manager write path with the derived pins).
func fitmentRecord(t *testing.T, w *casesWorld, caseID, workItemID id.UUID, key string, mutation func(*casesAPI.RecordFitmentEvidenceCommand)) casesAPI.WorkOutcome {
	t.Helper()
	command := casesAPI.RecordFitmentEvidenceCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		PositionKey: "WF2B196K666AA", CatalogNode: "Фильтр салона", Article: "WF2B196K666AA",
		LinkType: casesDomain.FitmentLinkOriginal, Chain: []string{"Фильтр салона (оригинальная позиция)"},
		CatalogVehicleIDs: []string{"ABCP-12345"},
		SourceSystem:      casesDomain.FitmentSourceAbcpCatalog, SourceRecordID: "rec-" + key, SourceVersion: "v2026.10",
		SourceObservedAt: casesClock.Now().Add(-2 * time.Hour),
		IdempotencyKey:   "fitment-" + key, Now: casesClock.Now(),
	}
	if mutation != nil {
		mutation(&command)
	}
	outcome, err := w.cases.RecordFitmentEvidence(context.Background(), command)
	if err != nil {
		t.Fatalf("the honest evidence record must land: %v", err)
	}
	return outcome
}

// fitmentPositionOf finds one position of the live fitment view.
func fitmentPositionOf(t *testing.T, view casesAPI.CaseView, positionKey string) casesDomain.FitmentPositionView {
	t.Helper()
	for _, position := range view.Fitment.Fitment.Positions {
		if position.PositionKey == positionKey {
			return position
		}
	}
	t.Fatalf("the fitment panel must carry the %s position, got %+v", positionKey, view.Fitment.Fitment.Positions)
	return casesDomain.FitmentPositionView{}
}

// TestFitmentMatrixRowsBySpec walks the Story 4.4 matrix over the live
// contour of the world 745: the fitment-policy-bearing 5.0.0 case pin,
// the honest case-level unknown of the absent proofs, the LIVE record
// command (the guard refusals BEFORE SQL, the derived pins, the monotonic
// numbers, the idempotent replay), the read-time state ladder (the proven
// original and cross chains, the researched not-found, the supersede by
// the new number, the conflicted pair, the ambiguous question, the stale
// states of the age and BOTH input pins) and the RLS blindness.
func TestFitmentMatrixRowsBySpec(t *testing.T) {
	w := newCasesWorld(t, 745)
	ctx := context.Background()
	fitmentSealPackFive(t, w, w.employeeID)

	// The ingestion of the matrix case (the world pins 5.0.0 — the case
	// row pins the fitment-policy-bearing pack).
	ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-fitment-1", "sess-fitment-1", `{"call":"inbound-call","from":"+79000000785"}`))
	if err != nil {
		t.Fatal(err)
	}
	result := acceptedOutcome(t, ingest)
	caseID := result.CaseID
	ingestDetail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ingestDetail.WorkItems) != 1 {
		t.Fatalf("expected the initial work item, got %d", len(ingestDetail.WorkItems))
	}
	workItemID := ingestDetail.WorkItems[0].WorkItemID

	// row-1: the case pins the fitment-policy-bearing pack 5.0.0 — the
	// fitment panel is defined, NO evidence exists: the honest case-level
	// unknown word with the research recommendation (AC-10 — the absence
	// is never a negative or a positive), the constant number-match word,
	// the price note and the scope word.
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.PackID != "auto-parts" || detail.Case.PackVersion != "5.0.0" {
		t.Fatalf("the case must pin auto-parts@5.0.0, got %s@%s", detail.Case.PackID, detail.Case.PackVersion)
	}
	if !detail.Fitment.SchemaDefined || detail.Fitment.Note != "" {
		t.Fatalf("the schema-defined fitment panel must carry no absence note, got %+v", detail.Fitment)
	}
	if detail.Fitment.Fitment.CaseUnknownWord != casesDomain.FitmentUnknownWord {
		t.Fatalf("the honest case-level unknown word expected, got %q", detail.Fitment.Fitment.CaseUnknownWord)
	}
	if len(detail.Fitment.Fitment.CaseHandoffWords) != 1 || detail.Fitment.Fitment.CaseHandoffWords[0] != casesDomain.HandoffReasonFitmentResearch {
		t.Fatalf("the research recommendation of the absent proof expected, got %v", detail.Fitment.Fitment.CaseHandoffWords)
	}
	if detail.Fitment.Fitment.NumberMatchWord != casesDomain.PartNumberMatchIsNotFitmentText {
		t.Fatalf("the constant number-match word expected, got %q", detail.Fitment.Fitment.NumberMatchWord)
	}
	if detail.Fitment.Fitment.PriceNote != casesDomain.FitmentPriceNote || detail.Fitment.Fitment.ScopeWord != casesDomain.FitmentScopeWord {
		t.Fatalf("the price note and the scope word expected")
	}
	if len(detail.Fitment.Fitment.Positions) != 0 {
		t.Fatalf("no positions exist before the first record, got %d", len(detail.Fitment.Fitment.Positions))
	}

	// row-2: the part request material of the case (the confirmed kind —
	// the PartRequestDigest pins the confirmed set the evidence derives
	// its request pin from).
	kindOutcome := partRecordFact(t, w, caseID, workItemID, "part.kind", []byte(`"фильтр салона"`), "fitment-fact-kind-1")
	if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: kindOutcome.FactID,
		VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		Note: "Клиент назвал деталь в разговоре", IdempotencyKey: "fitment-confirm-kind-1", Now: casesClock.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	requestDigestBefore := detail.PartRequest.Part.PartRequestDigest

	// row-3: the LIVE record of the found original proof → the proven
	// state with the FitmentDigest, the material (the node, the article,
	// the catalog vehicle ids, the source traceability) and BOTH pins
	// derived from the LIVE derivations (the operator never inputs them).
	outcome := fitmentRecord(t, w, caseID, workItemID, "1", nil)
	if outcome.FitmentEvidenceID == outcome.FitmentEvidenceID && len(outcome.FitmentDigest) != 64 {
		t.Fatalf("the outcome carries the canonical digest, got %+v", outcome)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	proven := fitmentPositionOf(t, detail, "WF2B196K666AA")
	if proven.Status != casesDomain.FitmentStatusProven || proven.StatusWord != casesDomain.FitmentProvenWord {
		t.Fatalf("the proven state expected, got %+v", proven)
	}
	if len(proven.FitmentDigest) != 64 || proven.FitmentDigest != outcome.FitmentDigest {
		t.Fatalf("the position carries the canonical digest of the outcome, got %q vs %q", proven.FitmentDigest, outcome.FitmentDigest)
	}
	if len(proven.Evidences) != 1 || proven.Evidences[0].EvidenceNo != 1 {
		t.Fatalf("the first evidence number expected, got %+v", proven.Evidences)
	}
	if proven.Evidences[0].PartRequestDigestPin != requestDigestBefore {
		t.Fatalf("the request pin derives from the LIVE derivation, got %q vs %q", proven.Evidences[0].PartRequestDigestPin, requestDigestBefore)
	}
	if len(proven.Evidences[0].ProfileDigestPin) != 64 {
		t.Fatalf("the profile pin derives from the LIVE derivation (the empty confirmed set digests the honest empty object), got %q", proven.Evidences[0].ProfileDigestPin)
	}
	if !proven.Evidences[0].Found || proven.Evidences[0].CatalogNode != "Фильтр салона" || proven.Evidences[0].Article != "WF2B196K666AA" {
		t.Fatalf("the found material carries the node and the article, got %+v", proven.Evidences[0])
	}

	// row-4: the byte-stable idempotent replay — the SAME command with the
	// SAME key returns the stored outcome, no second row lands.
	replayOutcome := fitmentRecord(t, w, caseID, workItemID, "1", nil)
	if replayOutcome.FitmentEvidenceID != outcome.FitmentEvidenceID {
		t.Fatalf("the replay returns the stored outcome, got %+v vs %+v", replayOutcome, outcome)
	}
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	proven = fitmentPositionOf(t, detail, "WF2B196K666AA")
	if len(proven.Evidences) != 1 {
		t.Fatalf("the replay writes nothing (the append-only row stays ONE), got %d", len(proven.Evidences))
	}

	// row-5: the cross chain proof of the analog position (the whole chain
	// rides the record, the depth 2 stays within the pack policy).
	fitmentRecord(t, w, caseID, workItemID, "2", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "B330PR"
		command.Article = "B330PR"
		command.LinkType = casesDomain.FitmentLinkCross
		command.Chain = []string{"Фильтр салона (оригинальная позиция)", "кросс-запись источника", "аналог B330PR"}
		command.IdempotencyKey = "fitment-2"
		command.SourceRecordID = "rec-2"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	cross := fitmentPositionOf(t, detail, "B330PR")
	if cross.Status != casesDomain.FitmentStatusProven {
		t.Fatalf("the cross chain proof stays proven, got %+v", cross)
	}
	if len(cross.Evidences[0].Chain) != 3 {
		t.Fatalf("the WHOLE cross chain rides the record (AC-4), got %v", cross.Evidences[0].Chain)
	}

	// row-6: the guard refusals LIVE — BEFORE any SQL with the dictionary
	// words (nothing is silently fixed). The dishonest material never
	// lands: the foreign link type, the foreign source, the
	// non-canonical position key, the original with the long chain, the
	// cross chain over the pack depth, the found record without the node,
	// the missing source record id.
	for _, refusal := range []struct {
		name string
		mut  func(*casesAPI.RecordFitmentEvidenceCommand)
		rule string
		word string
	}{
		{"foreign link type", func(c *casesAPI.RecordFitmentEvidenceCommand) { c.LinkType = "similar" }, "fitment_link_type_out_of_dictionary", "Domain Policy пака"},
		{"foreign source", func(c *casesAPI.RecordFitmentEvidenceCommand) { c.SourceSystem = "web_search" }, "fitment_source_out_of_dictionary", "Domain Policy пака"},
		{"non-canonical position key", func(c *casesAPI.RecordFitmentEvidenceCommand) { c.PositionKey = "wf 2b196k666 aa" }, "fitment_position_key_not_canonical", "канонизируйте"},
		{"original with the long chain", func(c *casesAPI.RecordFitmentEvidenceCommand) { c.Chain = []string{"a", "b"} }, "fitment_original_chain_shape", "один узел"},
		{"cross chain over the depth", func(c *casesAPI.RecordFitmentEvidenceCommand) {
			c.LinkType = casesDomain.FitmentLinkCross
			c.Chain = []string{"a", "b", "c", "d", "e"}
		}, "fitment_cross_chain_too_long", "слишком длинная cross-цепочка"},
		{"found without the node", func(c *casesAPI.RecordFitmentEvidenceCommand) { c.CatalogNode = "" }, "fitment_catalog_node_required", "каталогический узел"},
		{"missing source record id", func(c *casesAPI.RecordFitmentEvidenceCommand) { c.SourceRecordID = "" }, "fitment_source_record_id_required", "трассируемость"},
	} {
		command := casesAPI.RecordFitmentEvidenceCommand{
			Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
			PositionKey: "WF2B196K666AA", CatalogNode: "Фильтр салона", Article: "WF2B196K666AA",
			LinkType: casesDomain.FitmentLinkOriginal, Chain: []string{"Фильтр салона (оригинальная позиция)"},
			CatalogVehicleIDs: []string{"ABCP-12345"},
			SourceSystem:      casesDomain.FitmentSourceAbcpCatalog, SourceRecordID: "rec-x", SourceVersion: "v2026.10",
			SourceObservedAt: casesClock.Now().Add(-2 * time.Hour),
			IdempotencyKey:   "fitment-refusal-" + refusal.name, Now: casesClock.Now(),
		}
		refusal.mut(&command)
		_, err := w.cases.RecordFitmentEvidence(ctx, command)
		if err == nil {
			t.Fatalf("%s: the dishonest record is accepted", refusal.name)
		}
		rule, ok := err.(casesDomain.WorkRuleError)
		if !ok {
			t.Fatalf("%s: the refusal is not a WorkRuleError: %v", refusal.name, err)
		}
		// The app wraps the domain guard into the command rule (the rule()
		// helper): the nested dictionary rule travels INSIDE the honest
		// reason text together with the dictionary word.
		if rule.Rule != "record_fitment_evidence" {
			t.Fatalf("%s: rule = %q, want the command rule", refusal.name, rule.Rule)
		}
		if !strings.Contains(rule.Reason, refusal.rule) || !strings.Contains(rule.Reason, refusal.word) {
			t.Fatalf("%s: the honest refusal must carry the nested rule %q and the word %q, got %q", refusal.name, refusal.rule, refusal.word, rule.Reason)
		}
	}

	// row-7: the researched not-found record of its own position — the
	// honest researched unknown (NEVER «неприменимо», AC-10); the position
	// stays out of the confirmed-fitting candidates.
	fitmentRecord(t, w, caseID, workItemID, "3", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "C330PR"
		command.Article = ""
		command.CatalogNode = ""
		command.CatalogVehicleIDs = nil
		command.IdempotencyKey = "fitment-3"
		command.SourceRecordID = "rec-3"
		command.SourceSystem = casesDomain.FitmentSourceLaximoCatalog
		command.SourceVersion = "v3.11"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	researched := fitmentPositionOf(t, detail, "C330PR")
	if researched.Status != casesDomain.FitmentStatusUnknown || researched.StatusWord != casesDomain.FitmentUnknownResearchedWord {
		t.Fatalf("the researched unknown expected, got %+v", researched)
	}
	if researched.FitmentDigest != "" {
		t.Fatalf("the researched unknown carries no digest (the position is not proven)")
	}
	if strings.Contains(strings.ToLower(researched.StatusWord), "неприменимо") {
		t.Fatalf("the researched unknown must never become «неприменимо»")
	}

	// row-8: the conflicted pair of the two sources — the found proof of
	// ABCP against the not-found proof of Laximo (the CURRENT records):
	// BOTH proofs stay in the material, the position is never offered as
	// fitting, the unresolved_contradiction recommendation rides the
	// 2.13 dictionary.
	fitmentRecord(t, w, caseID, workItemID, "4", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "D330PR"
		command.Article = "D330PR"
		command.IdempotencyKey = "fitment-4"
		command.SourceRecordID = "rec-4"
	})
	fitmentRecord(t, w, caseID, workItemID, "5", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "D330PR"
		command.Article = ""
		command.CatalogNode = ""
		command.CatalogVehicleIDs = nil
		command.IdempotencyKey = "fitment-5"
		command.SourceRecordID = "rec-5"
		command.SourceSystem = casesDomain.FitmentSourceLaximoCatalog
		command.SourceVersion = "v3.11"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	conflicted := fitmentPositionOf(t, detail, "D330PR")
	if conflicted.Status != casesDomain.FitmentStatusConflicted || conflicted.StatusWord != casesDomain.FitmentConflictedSourcesWord {
		t.Fatalf("the conflicted state expected, got %+v", conflicted)
	}
	if len(conflicted.Evidences) != 2 {
		t.Fatalf("BOTH proofs stay in the material (AC-9), got %d", len(conflicted.Evidences))
	}
	if len(conflicted.HandoffWords) != 1 || conflicted.HandoffWords[0] != casesDomain.HandoffReasonUnresolvedContradiction {
		t.Fatalf("the unresolved_contradiction recommendation expected, got %v", conflicted.HandoffWords)
	}

	// row-9: the ambiguous record — several catalog vehicle ids: the most
	// probable modification is never selected, the minimal question of the
	// FIRST not-confirmed clarifiable attribute (vehicle.trim — AC-5).
	fitmentRecord(t, w, caseID, workItemID, "6", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "E330PR"
		command.Article = "E330PR"
		command.CatalogVehicleIDs = []string{"ABCP-12345", "ABCP-12346"}
		command.IdempotencyKey = "fitment-6"
		command.SourceRecordID = "rec-6"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	ambiguous := fitmentPositionOf(t, detail, "E330PR")
	if ambiguous.Status != casesDomain.FitmentStatusAmbiguous || ambiguous.StatusWord != casesDomain.FitmentAmbiguousWord {
		t.Fatalf("the ambiguous state expected, got %+v", ambiguous)
	}
	if question, ok := ambiguous.Questions["vehicle.trim"]; !ok || !strings.Contains(question, "модификация") {
		t.Fatalf("the minimal question of the missing attribute expected, got %v", ambiguous.Questions)
	}

	// row-10: the supersede by the NEW number — the second record of the
	// same source of the position supersedes the first (the current
	// record is the maximal number one, never an update).
	fitmentRecord(t, w, caseID, workItemID, "7", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "F330PR"
		command.Article = "F330PR"
		command.IdempotencyKey = "fitment-7"
		command.SourceRecordID = "rec-7"
	})
	fitmentRecord(t, w, caseID, workItemID, "8", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "F330PR"
		command.Article = ""
		command.CatalogNode = ""
		command.CatalogVehicleIDs = nil
		command.IdempotencyKey = "fitment-8"
		command.SourceRecordID = "rec-8"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	superseded := fitmentPositionOf(t, detail, "F330PR")
	if superseded.Status != casesDomain.FitmentStatusUnknown {
		t.Fatalf("the superseding not-found record wins (the researched unknown), got %+v", superseded)
	}
	if len(superseded.Evidences) != 1 || superseded.Evidences[0].EvidenceNo != 2 {
		t.Fatalf("the current record of the source is the maximal number one, got %+v", superseded.Evidences)
	}

	// row-11: the stale state by the REQUEST pin — the new version of the
	// confirmed part kind changes the PartRequestDigest: EVERY dependent
	// evidence turns stale with the honest words (AC-11 — a new version
	// of ANY input expires the dependent proofs read-time).
	partRecordFact(t, w, caseID, workItemID, "part.kind", []byte(`"салонный фильтр"`), "fitment-fact-kind-2")
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PartRequest.Part.PartRequestDigest == requestDigestBefore {
		t.Fatal("the new version of the confirmed kind changes the PartRequestDigest")
	}
	provenBefore := fitmentPositionOf(t, detail, "WF2B196K666AA")
	if provenBefore.Status != casesDomain.FitmentStatusStale {
		t.Fatalf("the new request version expires the dependent proof (AC-11), got %+v", provenBefore)
	}
	if len(provenBefore.Evidences[0].StaleWords) != 1 || provenBefore.Evidences[0].StaleWords[0] != casesDomain.FitmentStalePartRequestWord {
		t.Fatalf("the request stale word expected, got %v", provenBefore.Evidences[0].StaleWords)
	}
	if len(provenBefore.Words) == 0 || provenBefore.Words[0] != casesDomain.FitmentStaleRecheckWord {
		t.Fatalf("the re-check word of the stale state expected, got %v", provenBefore.Words)
	}

	// row-12: the fresh record with the ACTUAL pins restores the proven
	// state (AC-11 — the new record carries the current versions).
	fitmentRecord(t, w, caseID, workItemID, "9", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.IdempotencyKey = "fitment-9"
		command.SourceRecordID = "rec-9"
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if restored := fitmentPositionOf(t, detail, "WF2B196K666AA"); restored.Status != casesDomain.FitmentStatusProven {
		t.Fatalf("the fresh record with the actual pins restores proven, got %+v", restored)
	}

	// row-13: the stale state by the AGE — the record whose received time
	// is over the pack TTL (the clock fixture: the command lands with the
	// past Now, the view clock stays fixed).
	fitmentRecord(t, w, caseID, workItemID, "10", func(command *casesAPI.RecordFitmentEvidenceCommand) {
		command.PositionKey = "G330PR"
		command.Article = "G330PR"
		command.IdempotencyKey = "fitment-10"
		command.SourceRecordID = "rec-10"
		// The world clock of the case service is the releaseClock (the
		// fixed 2026-10-08): the command lands with the received time over
		// the pack TTL of that clock (the clock fixture — the view reads
		// the same fixed moment).
		command.Now = releaseClock.Now().Add(-604801 * time.Second)
	})
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	aged := fitmentPositionOf(t, detail, "G330PR")
	if aged.Status != casesDomain.FitmentStatusStale {
		t.Fatalf("the over-TTL record turns stale, got %+v", aged)
	}
	if len(aged.Evidences[0].StaleWords) == 0 || aged.Evidences[0].StaleWords[0] != casesDomain.FitmentStaleAgeWord {
		t.Fatalf("the age stale word expected, got %v", aged.Evidences[0].StaleWords)
	}

	// row-14: the RLS blindness — the foreign principal never reads the
	// case view of the matrix tenant (the generic refusal without
	// disclosure).
	foreignActor := identityDomain.Principal{ID: nextFitmentProbeID()}
	if _, err := w.cases.CaseView(ctx, foreignActor, continuationForeignTenant, caseID); err == nil ||
		!(errors.Is(err, casesDomain.ErrNotFound) || errors.Is(err, identityDomain.ErrNotFound)) {
		t.Fatalf("the foreign principal must never read the case view (the membership blindness), got %v", err)
	}
}

// TestFitmentPanelDegradesWithoutPolicy pins the honest degradation of
// the 4.0.0 pin: the pack carries the part request schema but NO fitment
// policy — the fitment panel is absent with the honest word, the read
// NEVER fails (the part request panel stays defined — the control).
func TestFitmentPanelDegradesWithoutPolicy(t *testing.T) {
	w := newCasesWorld(t, 746)
	ctx := context.Background()
	partSealPackFour(t, w, w.employeeID)
	ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-fitment-degrade-1", "sess-fitment-degrade-1", `{"call":"inbound-call","from":"+79000000786"}`))
	if err != nil {
		t.Fatal(err)
	}
	caseID := acceptedOutcome(t, ingest).CaseID
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.PackVersion != "4.0.0" {
		t.Fatalf("the degradation case pins the 4.0.0 pack, got %s", detail.Case.PackVersion)
	}
	if !detail.PartRequest.SchemaDefined {
		t.Fatal("the part request schema of 4.0.0 stays defined (the control)")
	}
	if detail.Fitment.SchemaDefined || detail.Fitment.Note != casesAPI.FitmentSchemaNoteAbsent {
		t.Fatalf("the fitment panel must degrade with the honest word, got %+v", detail.Fitment)
	}
}
