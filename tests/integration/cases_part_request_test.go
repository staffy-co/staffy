//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	casesAPI "github.com/staffy-co/staffy/internal/cases/api"
	casesDomain "github.com/staffy-co/staffy/internal/cases/domain"
	employeeAPI "github.com/staffy-co/staffy/internal/employee/api"
	employeeDomain "github.com/staffy-co/staffy/internal/employee/domain"
	identityDomain "github.com/staffy-co/staffy/internal/identity/domain"
	"github.com/staffy-co/staffy/internal/platform/id"
)

// The Story 4.3 integration matrix: the part request classification over
// the LIVE contour. The world of the matrix (the seed 741 — a free slot
// of the retro-3-3 register; the probe family 881xx continues the 88xxx
// c2xx family of Epic 4) seals the auto-parts@4.0.0 pin through the full
// live release chain (the pin chain 2.0.0 → 3.0.0 → the compatible
// additive upgrade 4.0.0 with the part request schema, the
// assemble/version/seal, the fresh profile approval and the new acting
// admission decision) BEFORE the case ingestion — the case row pins the
// schema-bearing pack and every surface derives over it. The matrix walks
// the mode ladder LIVE (named_part → original_vs_analog → the symptom
// dominance), the exact repeat with the honest number flags, the
// confirmation/digest/stale/conflicting chain, the several-parts word of
// the existing split, the boundary handoff with the 00019 word through
// the extended request path, the RLS blindness and the honest degradation
// of the 3.0.0 pin (the pack without the part request schema).

// partSeedSeq keeps the seeded probe rows globally unique.
var partSeedSeq int64

// nextPartProbeID mints one globally unique probe id of the matrix.
func nextPartProbeID() id.UUID {
	partSeedSeq++
	return id.MustParse(fmt.Sprintf("019a2f4c-6000-7000-8000-%012d", 88100+partSeedSeq))
}

// partSealPackFour seals the auto-parts@4.0.0 pin through the live
// release chain: the incompatible 1.0.0→2.0.0 pin (confirmed), the
// compatible 2.0.0→3.0.0 upgrade (the identification schema), the
// compatible 3.0.0→4.0.0 upgrade (the additive part request schema —
// Changed [part_request]), the assemble/version/seal of the new
// manifest, the fresh profile approval over the new link and the new
// acting admission decision.
func partSealPackFour(t *testing.T, w *casesWorld, employee id.UUID) employeeDomain.ManifestLink {
	t.Helper()
	ctx := context.Background()
	// The three-step pin chain: 2.0.0 (confirmed incompatible) → 3.0.0
	// (compatible additive identification) → 4.0.0 (compatible additive
	// part request schema).
	current, err := w.employee.Read(ctx, w.owner, w.tenant, employee)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.employee.Pin(ctx, employeeAPI.PinCommand{
		Actor: w.owner, TenantID: w.tenant, EmployeeID: employee,
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c211"),
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
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c212"),
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
		CorrelationID:   id.MustParse("01994f64-6c00-7000-8000-00000000c213"),
		ExpectedVersion: current.Version, PackID: "auto-parts", PackVersion: "4.0.0",
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
	version := w.createVersion(t, employee, draft.Revision, "Пин 4.0.0: схема запроса детали")
	manifest, err := w.seal(t, employee, version.Version, draft.Revision+1)
	if err != nil {
		t.Fatal(err)
	}
	link := employeeDomain.ManifestLink{ManifestID: manifest.ManifestID, ConfigVersion: manifest.ConfigVersion, Digest: manifest.Digest}
	// The fresh profile approval over the new link (the 4.0.0 pack carries
	// the same journeys as 3.0.0 — no new coverage cells; the
	// identification cells of the vehicle seal stay the honest covered
	// additions).
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
		CorrelationID: id.MustParse("01994f64-6c00-7000-8000-00000000c214"),
		Stage:         employeeDomain.StageSynthetic,
	}); err != nil {
		t.Fatal(err)
	}
	return link
}

// partRecordFact records one part request material fact through the live
// operator command (the full provenance path of 2.2).
func partRecordFact(t *testing.T, w *casesWorld, caseID, workItemID id.UUID, key string, value []byte, keyName string) casesAPI.WorkOutcome {
	t.Helper()
	outcome, err := w.cases.RecordFact(context.Background(), casesAPI.RecordFactCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, WorkItemID: workItemID,
		MaterialKey: key, Value: value, SourceType: casesDomain.FactSourceOperator, SourceID: keyName,
		Reason: "Материал запроса детали", IdempotencyKey: keyName, Now: casesClock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// partAttributeOf finds one attribute of the live part request view.
func partAttributeOf(t *testing.T, view casesAPI.CaseView, key string) casesAPI.PartAttributeView {
	t.Helper()
	for _, attribute := range view.PartRequest.Part.Attributes {
		if attribute.MaterialKey == key {
			return attribute
		}
	}
	t.Fatalf("the part request panel must carry the %s attribute, got %+v", key, view.PartRequest.Part.Attributes)
	return casesAPI.PartAttributeView{}
}

// TestPartRequestMatrixRowsBySpec walks the Story 4.3 matrix over the
// live contour of the world c210: the schema-bearing 4.0.0 case pin, the
// honest empty material with the dictionary hint, the LIVE mode ladder
// (named_part → original_vs_analog → the symptom dominance), the exact
// repeat with the honest number flags (the raw preserved verbatim, the
// normalized rendering, the constant number-match word), the
// confirmation/digest chain, the stale word of the new version, the
// conflicting refusal to pick a variant, the several-parts word of the
// existing split, the boundary handoff with the 00019 word, the RLS
// blindness and the honest degradation of the 3.0.0 pin.
func TestPartRequestMatrixRowsBySpec(t *testing.T) {
	w := newCasesWorld(t, 741)
	ctx := context.Background()
	partSealPackFour(t, w, w.employeeID)

	// The ingestion of the matrix case (the acceptedCase helper pins the
	// 1.0.0 world pin — this world pins 4.0.0, so the ingest runs inline).
	ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-part-1", "sess-part-1", `{"call":"inbound-call","from":"+79000000777"}`))
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

	// row-1: the case pins the schema-bearing pack 4.0.0 — the part
	// request panel is defined, the empty material carries the honest
	// unstructured word with the dictionary hint (never a guess).
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.PackID != "auto-parts" || detail.Case.PackVersion != "4.0.0" {
		t.Fatalf("the case must pin auto-parts@4.0.0, got %s@%s", detail.Case.PackID, detail.Case.PackVersion)
	}
	if !detail.PartRequest.SchemaDefined || detail.PartRequest.Note != "" {
		t.Fatalf("the schema-defined part request panel must carry no absence note, got %+v", detail.PartRequest)
	}
	if detail.PartRequest.Part.Mode != "" || detail.PartRequest.Part.UnstructuredWord != casesDomain.PartRequestUnstructuredWord {
		t.Fatalf("the empty material carries the honest unstructured word, got %+v", detail.PartRequest.Part)
	}
	if len(detail.PartRequest.Part.SchemaDictionary) != 10 {
		t.Fatalf("the dictionary hint carries the ten material keys of the pack, got %d", len(detail.PartRequest.Part.SchemaDictionary))
	}
	if detail.PartRequest.Part.ConfidenceWord != "материал без подтверждения" {
		t.Fatalf("the honest confidence word of the empty material expected, got %q", detail.PartRequest.Part.ConfidenceWord)
	}

	// row-2: the named_part material lands through the operator command;
	// the mode derives deterministically with the transparent basis word;
	// the missing keys of the SELECTED mode carry the minimal questions
	// (both keys are unconfirmed yet — both are honestly missing, AC-3).
	kindOutcome := partRecordFact(t, w, caseID, workItemID, "part.kind", []byte(`"щётки стеклоочистителя"`), "part-fact-kind-1")
	partRecordFact(t, w, caseID, workItemID, "part.quantity", []byte(`"2"`), "part-fact-quantity-1")
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PartRequest.Part.Mode != casesDomain.PartModeNamedPart {
		t.Fatalf("named_part expected over the kind+quantity material, got %q", detail.PartRequest.Part.Mode)
	}
	if detail.PartRequest.Part.ModeBasisWord != "основание: названа деталь" {
		t.Fatalf("the transparent basis word expected, got %q", detail.PartRequest.Part.ModeBasisWord)
	}
	if len(detail.PartRequest.Part.Completeness.MissingKeys) != 2 {
		t.Fatalf("both unconfirmed keys of the selected mode are honestly missing, got %v", detail.PartRequest.Part.Completeness.MissingKeys)
	}
	if detail.PartRequest.Part.Questions["part.quantity"] != "уточните: сколько нужно?" {
		t.Fatalf("the minimal quantity question expected, got %q", detail.PartRequest.Part.Questions["part.quantity"])
	}
	// The absent optional keys never appear (the honest absence).
	for _, missing := range detail.PartRequest.Part.Completeness.MissingKeys {
		if missing == "part.budget" || missing == "part.side" {
			t.Fatalf("the absent optional keys are honest absences, never missing, got %v", detail.PartRequest.Part.Completeness.MissingKeys)
		}
	}

	// row-3: the operator confirmation of the exact version — the
	// confirmed kind leaves the missing list (AC-3: the confirmed
	// material is never re-asked), the digest changes, the confidence
	// word renders the partial confirmation.
	if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseID, FactID: kindOutcome.FactID,
		VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		Note: "Клиент назвал деталь в разговоре", IdempotencyKey: "part-confirm-kind-1", Now: casesClock.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	digestBefore := detail.PartRequest.Part.PartRequestDigest
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	kind := partAttributeOf(t, detail, "part.kind")
	if kind.Status != casesDomain.FactStatusConfirmed || kind.Confirmation == nil {
		t.Fatalf("the operator confirmation derives confirmed, got %+v", kind)
	}
	if len(detail.PartRequest.Part.Completeness.MissingKeys) != 1 || detail.PartRequest.Part.Completeness.MissingKeys[0] != "part.quantity" {
		t.Fatalf("only the not-confirmed quantity stays missing (AC-3), got %v", detail.PartRequest.Part.Completeness.MissingKeys)
	}
	if detail.PartRequest.Part.PartRequestDigest == digestBefore || len(detail.PartRequest.Part.PartRequestDigest) != 64 {
		t.Fatalf("the PartRequestDigest must change over the confirmed set, got %q", detail.PartRequest.Part.PartRequestDigest)
	}
	if detail.PartRequest.Part.ConfidenceWord != "материал подтверждён частично: подтверждены 1 из 2 обязательных ключей выбранного режима" {
		t.Fatalf("the honest partial confidence word expected, got %q", detail.PartRequest.Part.ConfidenceWord)
	}

	// row-4: the preference «both» material re-derives the mode LIVE —
	// the choice request selects original_vs_analog (the ladder walks up
	// as the material accumulates; the read-time derivation never
	// writes).
	partRecordFact(t, w, caseID, workItemID, "part.preference", []byte(`"both"`), "part-fact-preference-1")
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PartRequest.Part.Mode != casesDomain.PartModeOriginalVsAnalog {
		t.Fatalf("original_vs_analog expected over the preference «both» material, got %q", detail.PartRequest.Part.Mode)
	}
	if detail.PartRequest.Part.ModeBasisWord != "основание: названа деталь; клиент просит выбор (значение both)" {
		t.Fatalf("the choice basis word expected, got %q", detail.PartRequest.Part.ModeBasisWord)
	}

	// row-5: the symptom material dominates ALWAYS — the mode re-derives
	// diagnostic over the accumulated material (the probabilistic guess
	// never becomes a catalog request, AC-7), the diagnostic boundary and
	// the candidate word render (the kind is present under the symptom),
	// the handoff recommendation carries the 00019 word.
	partRecordFact(t, w, caseID, workItemID, "part.symptom", []byte(`"грохот при торможении на кочках"`), "part-fact-symptom-1")
	detail, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.PartRequest.Part.Mode != casesDomain.PartModeDiagnostic {
		t.Fatalf("diagnostic expected once the symptom is recorded (the dominance), got %q", detail.PartRequest.Part.Mode)
	}
	boundaryWords := fmt.Sprintf("%+v", detail.PartRequest.Part.Boundaries)
	if !strings.Contains(boundaryWords, casesDomain.PartRequestDiagnosticBoundaryText) {
		t.Fatalf("the diagnostic boundary must render, got %s", boundaryWords)
	}
	if !strings.Contains(boundaryWords, casesDomain.PartRequestCandidateUnderSymptomText) {
		t.Fatalf("the candidate word of the named part under the symptom must render, got %s", boundaryWords)
	}
	handoffWords := fmt.Sprintf("%+v", detail.PartRequest.Part.HandoffWords)
	if !strings.Contains(handoffWords, casesDomain.HandoffReasonDiagnosticUncertainty) {
		t.Fatalf("the diagnostic boundary recommends the 00019 word diagnostic_uncertainty, got %s", handoffWords)
	}

	// row-6: the exact repeat case — the raw number (spaces, lower case)
	// is preserved verbatim, the normalized rendering carries the honest
	// flags, the constant number-match word renders (the match is not a
	// fitment proof, AC-4).
	ingestB, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-part-2", "sess-part-2", `{"call":"inbound-call","from":"+79000000778"}`))
	if err != nil {
		t.Fatal(err)
	}
	caseB := acceptedOutcome(t, ingestB).CaseID
	detailB, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	workItemB := detailB.WorkItems[0].WorkItemID
	partRecordFact(t, w, caseB, workItemB, "part.kind", []byte(`"фильтр салона"`), "part-fact-kind-b1")
	oemOutcome := partRecordFact(t, w, caseB, workItemB, "part.oem_number", []byte(`" wf 2b19 6k666 aa "`), "part-fact-oem-b1")
	detailB, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	if detailB.PartRequest.Part.Mode != casesDomain.PartModeExactRepeat {
		t.Fatalf("exact_repeat expected over the kind+oem material, got %q", detailB.PartRequest.Part.Mode)
	}
	oem := partAttributeOf(t, detailB, "part.oem_number")
	if oem.RawValue != " wf 2b19 6k666 aa " || oem.NumberChecks == nil || oem.NumberChecks.Normalized != "WF2B196K666AA" {
		t.Fatalf("the raw preservation and the normalized rendering expected, got %+v", oem)
	}
	if len(oem.NumberChecks.Words) != 0 || !oem.NumberChecks.AlphabetOK {
		t.Fatalf("the clean number carries no flags, got %+v", oem.NumberChecks)
	}
	if detailB.PartRequest.Part.NumberMatchWord != casesDomain.PartNumberMatchIsNotFitmentText {
		t.Fatalf("the constant number-match word must render, got %q", detailB.PartRequest.Part.NumberMatchWord)
	}

	// row-7: the cyrillic homoglyph of the number — the detection renders
	// the honest re-ask word, the rune is NEVER replaced (the VIN
	// discipline of 4.2), the attribute stays unverified.
	ingestC, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-part-3", "sess-part-3", `{"call":"inbound-call","from":"+79000000779"}`))
	if err != nil {
		t.Fatal(err)
	}
	caseC := acceptedOutcome(t, ingestC).CaseID
	detailC, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseC)
	if err != nil {
		t.Fatal(err)
	}
	workItemC := detailC.WorkItems[0].WorkItemID
	partRecordFact(t, w, caseC, workItemC, "part.kind", []byte(`"фильтр салона"`), "part-fact-kind-c1")
	partRecordFact(t, w, caseC, workItemC, "part.oem_number", []byte(`"WF2B19АK666AA"`), "part-fact-oem-c1")
	detailC, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseC)
	if err != nil {
		t.Fatal(err)
	}
	oemC := partAttributeOf(t, detailC, "part.oem_number")
	if oemC.NumberChecks == nil || len(oemC.NumberChecks.CyrillicAmbiguities) != 1 {
		t.Fatalf("the cyrillic homoglyph must be detected, got %+v", oemC.NumberChecks)
	}
	if oemC.NumberChecks.CyrillicAmbiguities[0].Rune != "А" || oemC.NumberChecks.CyrillicAmbiguities[0].LookAlike != "A" {
		t.Fatalf("the cyrillic А / latin A homoglyph expected, got %+v", oemC.NumberChecks.CyrillicAmbiguities)
	}
	if !strings.Contains(strings.Join(oemC.NumberChecks.Words, "; "), "сверьте номер посимвольно") {
		t.Fatalf("the honest re-ask word must render, got %v", oemC.NumberChecks.Words)
	}
	if !strings.Contains(oemC.NumberChecks.Normalized, "А") {
		t.Fatalf("the cyrillic rune stays verbatim in the normalized rendering (NEVER replaced), got %q", oemC.NumberChecks.Normalized)
	}
	if oemC.Status != casesDomain.FactStatusUnverified {
		t.Fatalf("the ambiguous number attribute stays unverified, got %s", oemC.Status)
	}

	// row-8: the new version after the confirmation makes the attribute
	// honestly stale — the confirmation binds the former version, the
	// stale word names it, the digest changes, the aggregate stale note
	// renders, the conflicting status derives the contradiction boundary
	// with the 00019 word and the honest confidence refusal.
	if _, err := w.cases.ConfirmFactVersion(ctx, casesAPI.ConfirmFactVersionCommand{
		Actor: w.workActor(), TenantID: w.tenant, CaseID: caseB, FactID: oemOutcome.FactID,
		VersionNo: 1, ConfirmerType: casesDomain.ConfirmerOperatorRelay,
		Note: "Клиент продиктовал номер в разговоре", IdempotencyKey: "part-confirm-oem-b1", Now: casesClock.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	digestConfirmed := ""
	detailB, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	digestConfirmed = detailB.PartRequest.Part.PartRequestDigest
	partRecordFact(t, w, caseB, workItemB, "part.oem_number", []byte(`"wf 2b19 6k666 ab"`), "part-fact-oem-b2")
	detailB, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseB)
	if err != nil {
		t.Fatal(err)
	}
	oemStale := partAttributeOf(t, detailB, "part.oem_number")
	if oemStale.Status != casesDomain.FactStatusConflicting {
		t.Fatalf("the new version after the confirmation conflicts (the 2.2 canon), got %s", oemStale.Status)
	}
	if oemStale.Confirmation == nil || oemStale.Confirmation.StaleWord != "подтверждение относилось к прежней версии (v1)" {
		t.Fatalf("the stale word must name the former version, got %+v", oemStale.Confirmation)
	}
	if detailB.PartRequest.Part.StaleNote != casesDomain.PartRequestStaleNote {
		t.Fatalf("the aggregate stale note must render, got %q", detailB.PartRequest.Part.StaleNote)
	}
	if detailB.PartRequest.Part.PartRequestDigest == digestConfirmed {
		t.Fatal("the PartRequestDigest must change when a confirmed attribute becomes stale")
	}
	boundaryB := fmt.Sprintf("%+v", detailB.PartRequest.Part.Boundaries)
	if !strings.Contains(boundaryB, casesDomain.PartRequestContradictionBoundaryText) {
		t.Fatalf("the contradiction boundary must render over the conflicting sources, got %s", boundaryB)
	}
	if !strings.Contains(fmt.Sprintf("%+v", detailB.PartRequest.Part.HandoffWords), casesDomain.HandoffReasonUnresolvedContradiction) {
		t.Fatalf("the contradiction recommends the 00019 word unresolved_contradiction, got %+v", detailB.PartRequest.Part.HandoffWords)
	}
	if !strings.Contains(detailB.PartRequest.Part.ConfidenceWord, "противоречия") {
		t.Fatalf("the honest confidence refusal over the contradiction expected, got %q", detailB.PartRequest.Part.ConfidenceWord)
	}

	// row-9: the several-parts word of the existing split — the classified
	// «поиск детали» item splits into two typed targets, the count of the
	// ACTIVE part-search items derives two, the honest word renders (ONE
	// structured set per case — the split recommendation of 2.2).
	ingestD, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-part-4", "sess-part-4", `{"call":"inbound-call","from":"+79000000780"}`))
	if err != nil {
		t.Fatal(err)
	}
	caseD := acceptedOutcome(t, ingestD).CaseID
	detailD, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseD)
	if err != nil {
		t.Fatal(err)
	}
	workItemD := detailD.WorkItems[0].WorkItemID
	classifyInput := w.workBaseInput(caseD, workItemD, 1)
	classifyInput.Reason, classifyInput.IdempotencyKey = "Клиент просит колодки и фильтр", "part-classify-1"
	if _, err := w.cases.ClassifyWorkItem(ctx, casesAPI.ClassifyCommand{
		WorkCommandInput: classifyInput, Type: "поиск детали", Title: "Колодки и фильтр",
	}); err != nil {
		t.Fatal(err)
	}
	detailD, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseD)
	if err != nil {
		t.Fatal(err)
	}
	if detailD.PartRequest.Part.SeveralPartsWord != "" {
		t.Fatalf("a single active part-search item carries no several-parts word, got %q", detailD.PartRequest.Part.SeveralPartsWord)
	}
	splitInput := w.workBaseInput(caseD, workItemD, 2)
	splitInput.Reason, splitInput.IdempotencyKey = "Два независимых намерения клиента", "part-split-1"
	if _, err := w.cases.SplitWorkItem(ctx, casesAPI.SplitCommand{
		WorkCommandInput: splitInput,
		Targets: []casesDomain.SplitTarget{
			{Type: "поиск детали", Title: "Тормозные колодки"},
			{Type: "поиск детали", Title: "Фильтр салона"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	detailD, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseD)
	if err != nil {
		t.Fatal(err)
	}
	if detailD.PartRequest.Part.SeveralPartsWord != casesDomain.PartRequestSeveralPartsText {
		t.Fatalf("the honest several-parts word must render over the two active items, got %q", detailD.PartRequest.Part.SeveralPartsWord)
	}

	// row-10: the safety boundary under the diagnostic uncertainty — the
	// safety-marker kind with the symptom stops the automatic selection
	// and recommends the 00019 word safety_critical; the operator creates
	// the boundary handoff with that word through the extended request
	// path LIVE (the full app machinery — the honest reason row of the
	// migration 00019). The facts land on the LIVE split target (the
	// source went terminal split — the fact scope discipline of 2.2).
	var liveItemD id.UUID
	for _, item := range detailD.WorkItems {
		if !item.Terminal && item.Type == "поиск детали" {
			liveItemD = item.WorkItemID
			break
		}
	}
	if !liveItemD.Valid() {
		t.Fatal("the split must leave a live part-search target")
	}
	partRecordFact(t, w, caseD, liveItemD, "part.kind", []byte(`"тормозные колодки"`), "part-fact-kind-d1")
	partRecordFact(t, w, caseD, liveItemD, "part.symptom", []byte(`"грохот при торможении"`), "part-fact-symptom-d1")
	detailD, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseD)
	if err != nil {
		t.Fatal(err)
	}
	boundaryD := fmt.Sprintf("%+v", detailD.PartRequest.Part.Boundaries)
	if !strings.Contains(boundaryD, casesDomain.PartRequestSafetyStopText) {
		t.Fatalf("the safety stop under the diagnostic uncertainty must render, got %s", boundaryD)
	}
	if !strings.Contains(fmt.Sprintf("%+v", detailD.PartRequest.Part.HandoffWords), casesDomain.HandoffReasonSafetyCritical) {
		t.Fatalf("the safety boundary recommends the 00019 word safety_critical, got %+v", detailD.PartRequest.Part.HandoffWords)
	}
	// The LIVE boundary handoff: the operator request with the
	// recommended word lands through the full path (the honest reason row
	// of 00019 in the live database).
	turnD := w.modeTurnEvent(t, caseD, "sess-part-4", "evt-part-4-turn", `{"say":"позовите специалиста"}`)
	if _, err := w.cases.RequestHuman(ctx, casesAPI.RequestHumanCommand{
		Actor: w.ownershipActor(), TenantID: w.tenant, CaseID: caseD,
		TurnRef: turnD, ExpectedModeVersion: 1,
		TargetRoles: []string{casesDomain.HandoffTargetRoleOperator}, AckDeadline: casesDomain.HandoffAck15m,
		Reason: casesDomain.HandoffReasonSafetyCritical, IdempotencyKey: "part-handoff-safety-1", Now: ownershipNow,
	}); err != nil {
		t.Fatalf("the boundary handoff with the safety_critical word must land through the extended path, got %v", err)
	}
	detailD, err = w.cases.CaseView(ctx, w.workActor(), w.tenant, caseD)
	if err != nil {
		t.Fatal(err)
	}
	if len(detailD.Handoff.Requests) != 1 || detailD.Handoff.Requests[0].Reason != casesDomain.HandoffReasonSafetyCritical {
		t.Fatalf("the boundary handoff reason row renders the 00019 word, got %+v", detailD.Handoff.Requests)
	}
	if detailD.Handoff.Requests[0].ReasonText != casesDomain.HandoffReasonText(casesDomain.HandoffReasonSafetyCritical) {
		t.Fatalf("the boundary reason carries its honest text, got %q", detailD.Handoff.Requests[0].ReasonText)
	}

	// row-11: the RLS blindness — the foreign principal never reads the
	// case view of the matrix tenant (the generic refusal without
	// disclosure).
	foreignActor := identityDomain.Principal{ID: nextPartProbeID()}
	if _, err := w.cases.CaseView(ctx, foreignActor, continuationForeignTenant, caseID); err == nil ||
		!(errors.Is(err, casesDomain.ErrNotFound) || errors.Is(err, identityDomain.ErrNotFound)) {
		t.Fatalf("the foreign principal must never read the case view (the membership blindness), got %v", err)
	}
}

// TestPartRequestPanelDegradesWithoutSchema pins the honest degradation
// of the 3.0.0 pin: the pack carries the identification schema but NO
// part request schema — the panel is absent with the honest word, the
// read NEVER fails (the vehicle seal of the 4.2 matrix pins 3.0.0).
func TestPartRequestPanelDegradesWithoutSchema(t *testing.T) {
	w := newCasesWorld(t, 742)
	ctx := context.Background()
	vehicleSealPackThree(t, w, w.employeeID)
	ingest, err := w.cases.IngestEvent(ctx, w.ingestCommand("evt-part-degrade-1", "sess-part-degrade-1", `{"call":"inbound-call","from":"+79000000781"}`))
	if err != nil {
		t.Fatal(err)
	}
	caseID := acceptedOutcome(t, ingest).CaseID
	detail, err := w.cases.CaseView(ctx, w.workActor(), w.tenant, caseID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Case.PackVersion != "3.0.0" {
		t.Fatalf("the degradation case pins the 3.0.0 pack, got %s", detail.Case.PackVersion)
	}
	if !detail.Vehicle.SchemaDefined {
		t.Fatal("the vehicle schema of 3.0.0 stays defined (the control)")
	}
	if detail.PartRequest.SchemaDefined || detail.PartRequest.Note != casesAPI.PartSchemaNoteAbsent {
		t.Fatalf("the part request panel must degrade with the honest word, got %+v", detail.PartRequest)
	}
}
